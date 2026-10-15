// Package daraja implements the payment Gateway port against Safaricom's
// Daraja API (Lipa Na M-Pesa Online / STK push).
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

type Client struct {
	SandboxURL    string
	ProductionURL string
	HTTP          *http.Client

	sf     singleflight.Group
	mu     sync.Mutex
	tokens map[string]cachedToken
	now    func() time.Time
}

type cachedToken struct {
	value   string
	expires time.Time
}

var _ app.Gateway = (*Client)(nil)

func New(sandboxURL, productionURL string) *Client {
	return &Client{
		SandboxURL: sandboxURL, ProductionURL: productionURL,
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		tokens: map[string]cachedToken{},
		now:    time.Now,
	}
}

func (c *Client) baseURL(ch domain.Channel) string {
	if ch.Environment == "production" {
		return c.ProductionURL
	}
	return c.SandboxURL
}

// ---------------------------------------------------------------- OAuth

// token returns a cached access token for the channel's app credentials,
// fetching one at most once concurrently per credential set.
func (c *Client) token(ctx context.Context, ch domain.Channel) (string, error) {
	key := ch.Environment + "|" + ch.ConsumerKey
	c.mu.Lock()
	t, ok := c.tokens[key]
	c.mu.Unlock()
	if ok && c.now().Before(t.expires) {
		return t.value, nil
	}
	v, err, _ := c.sf.Do(key, func() (any, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL(ch)+"/oauth/v1/generate?grant_type=client_credentials", nil)
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(ch.ConsumerKey, ch.ConsumerSecret)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("daraja oauth: %w", err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
			return nil, &app.RejectedError{Code: "invalid_credentials", Message: "Daraja rejected the channel's consumer key/secret"}
		}
		if resp.StatusCode != http.StatusOK {
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
		c.tokens[key] = cachedToken{out.AccessToken, c.now().Add(time.Duration(secs-60) * time.Second)}
		c.mu.Unlock()
		return out.AccessToken, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (c *Client) invalidate(ch domain.Channel) {
	c.mu.Lock()
	delete(c.tokens, ch.Environment+"|"+ch.ConsumerKey)
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
func (c *Client) post(ctx context.Context, ch domain.Channel, path string, in any) (int, []byte, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return 0, nil, err
	}
	for attempt := 0; ; attempt++ {
		tok, err := c.token(ctx, ch)
		if err != nil {
			return 0, nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL(ch)+path, bytes.NewReader(payload))
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
			c.invalidate(ch)
			continue
		}
		return resp.StatusCode, body, nil
	}
}

func password(ch domain.Channel, ts string) string {
	return base64.StdEncoding.EncodeToString([]byte(ch.ShortCode + ch.Passkey + ts))
}

func (c *Client) STKPush(ctx context.Context, ch domain.Channel, r app.STKPushRequest) (app.STKPushAck, error) {
	ts := c.now().In(eat).Format("20060102150405")
	txType, partyB := "CustomerPayBillOnline", ch.ShortCode
	if ch.Type == "till" {
		txType, partyB = "CustomerBuyGoodsOnline", ch.TillNumber
	}
	status, body, err := c.post(ctx, ch, "/mpesa/stkpush/v1/processrequest", map[string]string{
		"BusinessShortCode": ch.ShortCode,
		"Password":          password(ch, ts),
		"Timestamp":         ts,
		"TransactionType":   txType,
		"Amount":            strconv.FormatInt(r.Amount, 10),
		"PartyA":            r.Phone,
		"PartyB":            partyB,
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

func (c *Client) STKQuery(ctx context.Context, ch domain.Channel, checkoutRequestID string) (app.STKQueryResult, error) {
	ts := c.now().In(eat).Format("20060102150405")
	status, body, err := c.post(ctx, ch, "/mpesa/stkpushquery/v1/query", map[string]string{
		"BusinessShortCode": ch.ShortCode,
		"Password":          password(ch, ts),
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
