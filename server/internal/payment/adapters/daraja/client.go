// Package daraja implements the payment Gateway port against Safaricom's
// Daraja API (Lipa Na M-Pesa Online / STK push). Every request is made with
// the platform's own Daraja app: BusinessShortCode and Password come from
// the platform credentials, while PartyB routes the money to the tenant's
// destination.
package daraja

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/eslieh/paytrack-go/server/internal/payment/app"
	"github.com/eslieh/paytrack-go/server/internal/payment/domain"
)

// eat is Daraja's timestamp zone (Africa/Nairobi, no DST).
var eat = time.FixedZone("EAT", 3*60*60)

// Daraja error code returned by STK query while the customer has not acted yet.
const errStillProcessing = "500.001.1001"

// Credentials identify the platform's Daraja app.
type Credentials struct {
	BaseURL        string
	ConsumerKey    string
	ConsumerSecret string
	ShortCode      string // BusinessShortCode that signs every STK request
	Passkey        string
}

type Client struct {
	creds Credentials
	HTTP  *http.Client

	sf    singleflight.Group
	mu    sync.Mutex
	token cachedToken
	now   func() time.Time
}

type cachedToken struct {
	value   string
	expires time.Time
}

var _ app.Gateway = (*Client)(nil)

func New(creds Credentials) *Client {
	return &Client{creds: creds, HTTP: &http.Client{Timeout: 30 * time.Second}, now: time.Now}
}

// ---------------------------------------------------------------- OAuth

// accessToken returns the cached OAuth token, fetching a new one at most
// once concurrently.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	t := c.token
	c.mu.Unlock()
	if t.value != "" && c.now().Before(t.expires) {
		return t.value, nil
	}
	v, err, _ := c.sf.Do("token", func() (any, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.creds.BaseURL+"/oauth/v1/generate?grant_type=client_credentials", nil)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(c.creds.ConsumerKey, c.creds.ConsumerSecret)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("daraja oauth: %w", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if resp.StatusCode != http.StatusOK {
			// Misconfigured platform credentials are our fault, not the
			// tenant's request: report as an error, not a rejection.
			return nil, fmt.Errorf("daraja oauth: status %d: %s", resp.StatusCode, body)
		}
		var out struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   string `json:"expires_in"`
		}
		if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
			return nil, fmt.Errorf("daraja oauth: bad response: %s", body)
		}
		secs, _ := strconv.Atoi(out.ExpiresIn)
		if secs <= 0 {
			secs = 3599
		}
		// Refresh a minute early so a token never expires mid-request.
		c.mu.Lock()
		c.token = cachedToken{out.AccessToken, c.now().Add(time.Duration(secs-60) * time.Second)}
		c.mu.Unlock()
		return out.AccessToken, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (c *Client) invalidate() {
	c.mu.Lock()
	c.token = cachedToken{}
	c.mu.Unlock()
}

// ---------------------------------------------------------------- requests

type errorBody struct {
	RequestID    string `json:"requestId"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

// post sends an authenticated JSON request, retrying once with a fresh
// token if Daraja reports the token invalid.
func (c *Client) post(ctx context.Context, path string, in any) (int, []byte, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return 0, nil, err
	}
	for attempt := 0; ; attempt++ {
		tok, err := c.accessToken(ctx)
		if err != nil {
			return 0, nil, &app.NotSentError{Err: err}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.creds.BaseURL+path, bytes.NewReader(payload))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return 0, nil, fmt.Errorf("daraja %s: %w", path, err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			c.invalidate()
			continue
		}
		return resp.StatusCode, body, nil
	}
}

func (c *Client) password(ts string) string {
	return base64.StdEncoding.EncodeToString([]byte(c.creds.ShortCode + c.creds.Passkey + ts))
}

func (c *Client) STKPush(ctx context.Context, r app.STKPushRequest) (app.STKPushAck, error) {
	ts := c.now().In(eat).Format("20060102150405")
	status, body, err := c.post(ctx, "/mpesa/stkpush/v1/processrequest", map[string]string{
		"BusinessShortCode": c.creds.ShortCode,
		"Password":          c.password(ts),
		"Timestamp":         ts,
		"TransactionType":   r.TransactionType,
		"Amount":            strconv.FormatInt(r.Amount, 10),
		"PartyA":            r.Phone,
		"PartyB":            r.PartyB,
		"PhoneNumber":       r.Phone,
		"CallBackURL":       r.CallbackURL,
		"AccountReference":  r.AccountReference,
		"TransactionDesc":   r.Description,
	})
	if err != nil {
		return app.STKPushAck{}, err
	}

	var ok struct {
		MerchantRequestID   string `json:"MerchantRequestID"`
		CheckoutRequestID   string `json:"CheckoutRequestID"`
		ResponseCode        string `json:"ResponseCode"`
		ResponseDescription string `json:"ResponseDescription"`
		CustomerMessage     string `json:"CustomerMessage"`
	}
	if status == http.StatusOK && json.Unmarshal(body, &ok) == nil {
		if ok.ResponseCode == "0" && ok.CheckoutRequestID != "" {
			return app.STKPushAck{MerchantRequestID: ok.MerchantRequestID, CheckoutRequestID: ok.CheckoutRequestID, CustomerMessage: ok.CustomerMessage}, nil
		}
		return app.STKPushAck{}, &app.RejectedError{Code: ok.ResponseCode, Message: ok.ResponseDescription}
	}
	// 4xx with an error body is a definite rejection (validation, bad
	// credentials, ...). 5xx and anything unparseable is ambiguous.
	var eb errorBody
	if status >= 400 && status < 500 && json.Unmarshal(body, &eb) == nil && eb.ErrorCode != "" {
		return app.STKPushAck{}, &app.RejectedError{Code: eb.ErrorCode, Message: eb.ErrorMessage}
	}
	return app.STKPushAck{}, fmt.Errorf("daraja stk push: status %d: %s", status, body)
}

func (c *Client) STKQuery(ctx context.Context, checkoutRequestID string) (app.STKQueryResult, error) {
	ts := c.now().In(eat).Format("20060102150405")
	status, body, err := c.post(ctx, "/mpesa/stkpushquery/v1/query", map[string]string{
		"BusinessShortCode": c.creds.ShortCode,
		"Password":          c.password(ts),
		"Timestamp":         ts,
		"CheckoutRequestID": checkoutRequestID,
	})
	if err != nil {
		return app.STKQueryResult{}, err
	}
	var eb errorBody
	if status != http.StatusOK {
		if json.Unmarshal(body, &eb) == nil && eb.ErrorCode == errStillProcessing {
			return app.STKQueryResult{Pending: true}, nil
		}
		return app.STKQueryResult{}, fmt.Errorf("daraja stk query: status %d: %s", status, body)
	}
	var out struct {
		ResponseCode      string `json:"ResponseCode"`
		MerchantRequestID string `json:"MerchantRequestID"`
		CheckoutRequestID string `json:"CheckoutRequestID"`
		ResultCode        string `json:"ResultCode"`
		ResultDesc        string `json:"ResultDesc"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return app.STKQueryResult{}, fmt.Errorf("daraja stk query: bad response: %s", body)
	}
	code, err := strconv.Atoi(out.ResultCode)
	if out.ResponseCode != "0" || err != nil {
		return app.STKQueryResult{}, fmt.Errorf("daraja stk query: unexpected response: %s", body)
	}
	return app.STKQueryResult{Result: domain.Result{
		Code: code, Desc: out.ResultDesc, MerchantRequestID: out.MerchantRequestID, CheckoutRequestID: out.CheckoutRequestID,
	}}, nil
}
