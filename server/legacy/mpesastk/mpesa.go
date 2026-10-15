package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// Bank represents a bank payable via M-Pesa PayBill, identified by the
// bank's own PayBill shortcode (used as PartyB in the STK push request).
type Bank struct {
	Name      string `json:"name"`
	ShortCode string `json:"shortCode"`
}

// Banks lists the destination banks this app can route payments to. PartyB
// is set to the selected bank's PayBill shortcode; AccountReference carries
// the customer-supplied destination bank account number.
var Banks = []Bank{
	{"KCB Bank", "522522"},
	{"Equity Bank", "247247"},
	{"Co-operative Bank", "400200"},
	{"Stanbic Bank", "600100"},
	{"NCBA Bank", "880100"},
	{"Bank of Africa", "972900"},
	{"ABC Bank", "111777"},
	{"Bank of Baroda", "902363"},
}

var bankByShortCode = func() map[string]Bank {
	m := make(map[string]Bank, len(Banks))
	for _, b := range Banks {
		m[b.ShortCode] = b
	}
	return m
}()

func bankByCode(shortCode string) (Bank, bool) {
	b, ok := bankByShortCode[shortCode]
	return b, ok
}

var accountRefRe = regexp.MustCompile(`^[A-Za-z0-9\-_/. ]{1,12}$`)

// normalizeAccountRef validates the customer-supplied bank account number
// used as the STK push AccountReference. Daraja caps this field at 12 chars.
func normalizeAccountRef(ref string) (string, error) {
	ref = regexp.MustCompile(`^\s+|\s+$`).ReplaceAllString(ref, "")
	if !accountRefRe.MatchString(ref) {
		return "", fmt.Errorf("account number %q must be 1-12 characters (letters, digits, -_/. and spaces only)", ref)
	}
	return ref, nil
}

// PaymentType selects how the STK push is routed: to one of the curated
// Banks' PayBill, to an arbitrary business PayBill, or directly to a
// merchant's Till (Buy Goods).
type PaymentType string

const (
	PaymentTypeBank    PaymentType = "bank"
	PaymentTypePaybill PaymentType = "paybill"
	PaymentTypeTill    PaymentType = "till"
)

var shortCodeRe = regexp.MustCompile(`^\d{5,7}$`)

// normalizeShortCode validates a Safaricom PayBill or Till business number,
// issued as a 5-7 digit code.
func normalizeShortCode(code, label string) (string, error) {
	digits := regexp.MustCompile(`\D`).ReplaceAllString(code, "")
	if !shortCodeRe.MatchString(digits) {
		return "", fmt.Errorf("%s %q must be 5-7 digits", label, code)
	}
	return digits, nil
}

type MpesaClient struct {
	cfg        *Config
	httpClient *http.Client
}

func NewMpesaClient(cfg *Config) *MpesaClient {
	return &MpesaClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type authResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

// getAccessToken fetches a fresh OAuth token from Daraja using the app's
// consumer key/secret. Tokens are short-lived (~1hr) so we don't bother
// caching one for this simple app; a real service would cache until expiry.
func (m *MpesaClient) getAccessToken() (string, error) {
	url := m.cfg.BaseURL + "/oauth/v1/generate?grant_type=client_credentials"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(m.cfg.ConsumerKey, m.cfg.ConsumerSecret)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("access token request failed (%d): %s", resp.StatusCode, string(body))
	}

	var auth authResponse
	if err := json.Unmarshal(body, &auth); err != nil {
		return "", fmt.Errorf("parsing access token response: %w", err)
	}
	if auth.AccessToken == "" {
		return "", fmt.Errorf("access token response missing token: %s", string(body))
	}

	return auth.AccessToken, nil
}

type stkPushRequest struct {
	BusinessShortCode string `json:"BusinessShortCode"`
	Password          string `json:"Password"`
	Timestamp         string `json:"Timestamp"`
	TransactionType   string `json:"TransactionType"`
	Amount            string `json:"Amount"`
	PartyA            string `json:"PartyA"`
	PartyB            string `json:"PartyB"`
	PhoneNumber       string `json:"PhoneNumber"`
	CallBackURL       string `json:"CallBackURL"`
	AccountReference  string `json:"AccountReference"`
	TransactionDesc   string `json:"TransactionDesc"`
}

type StkPushResult struct {
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	CustomerMessage     string `json:"CustomerMessage"`
}

var phoneRe = regexp.MustCompile(`^254[17]\d{8}$`)

// normalizePhone accepts common Kenyan formats (07xx, 7xx, +254xx, 254xx)
// and returns the 2547xxxxxxxx / 2541xxxxxxxx format Daraja requires.
func normalizePhone(phone string) (string, error) {
	digits := regexp.MustCompile(`\D`).ReplaceAllString(phone, "")

	switch {
	case len(digits) == 9 && (digits[0] == '7' || digits[0] == '1'):
		digits = "254" + digits
	case len(digits) == 10 && digits[0] == '0':
		digits = "254" + digits[1:]
	case len(digits) == 12 && digits[:3] == "254":
		// already normalized
	default:
		return "", fmt.Errorf("unrecognized phone number format: %q", phone)
	}

	if !phoneRe.MatchString(digits) {
		return "", fmt.Errorf("phone number %q is not a valid Safaricom number", digits)
	}
	return digits, nil
}

// InitiateSTKPush prompts the given phone number to pay the given amount
// (KES) via M-Pesa's Lipa Na M-Pesa Online (STK Push) API.
//
// For PaymentTypeBank, destination is one of the known Banks shortcodes and
// accountReference is the destination bank account number; the push is sent
// as a PayBill (CustomerPayBillOnline). For PaymentTypeTill, destination is
// the merchant's Till number and the push is sent as Buy Goods
// (CustomerBuyGoodsOnline); accountReference is optional there and defaults
// to a generic label since Till transactions don't use it.
func (m *MpesaClient) InitiateSTKPush(rawPhone string, amount int, paymentType PaymentType, destination string, accountReference string) (*StkPushResult, error) {
	phone, err := normalizePhone(rawPhone)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be a positive whole number of KES")
	}

	var transactionType, partyB, accountRef, transactionDesc string

	switch paymentType {
	case PaymentTypeBank:
		bank, ok := bankByCode(destination)
		if !ok {
			return nil, fmt.Errorf("unknown destination bank shortcode %q", destination)
		}
		ref, err := normalizeAccountRef(accountReference)
		if err != nil {
			return nil, err
		}
		transactionType = "CustomerPayBillOnline"
		partyB = bank.ShortCode
		accountRef = ref
		transactionDesc = "Bank Deposit"

	case PaymentTypePaybill:
		paybill, err := normalizeShortCode(destination, "paybill number")
		if err != nil {
			return nil, err
		}
		ref, err := normalizeAccountRef(accountReference)
		if err != nil {
			return nil, err
		}
		transactionType = "CustomerPayBillOnline"
		partyB = paybill
		accountRef = ref
		transactionDesc = "Paybill Payment"

	case PaymentTypeTill:
		till, err := normalizeShortCode(destination, "till number")
		if err != nil {
			return nil, err
		}
		accountRef = "Till Payment"
		if accountReference != "" {
			ref, err := normalizeAccountRef(accountReference)
			if err != nil {
				return nil, err
			}
			accountRef = ref
		}
		transactionType = "CustomerBuyGoodsOnline"
		partyB = till
		transactionDesc = "Till Payment"

	default:
		return nil, fmt.Errorf("unknown payment type %q (must be %q, %q or %q)", paymentType, PaymentTypeBank, PaymentTypePaybill, PaymentTypeTill)
	}

	token, err := m.getAccessToken()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102150405")
	password := base64.StdEncoding.EncodeToString(
		[]byte(m.cfg.ShortCode + m.cfg.Passkey + timestamp),
	)

	reqBody := stkPushRequest{
		BusinessShortCode: m.cfg.ShortCode,
		Password:          password,
		Timestamp:         timestamp,
		TransactionType:   transactionType,
		Amount:            fmt.Sprintf("%d", amount),
		PartyA:            phone,
		PartyB:            partyB,
		PhoneNumber:       phone,
		CallBackURL:       m.cfg.CallbackURL,
		AccountReference:  accountRef,
		TransactionDesc:   transactionDesc,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := m.cfg.BaseURL + "/mpesa/stkpush/v1/processrequest"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending STK push request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result StkPushResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing STK push response: %w (raw: %s)", err, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("STK push request failed (%d): %s", resp.StatusCode, string(body))
	}

	return &result, nil
}
