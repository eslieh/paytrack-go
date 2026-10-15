// Package domain parses Daraja's STK callback payload. It is the
// anti-corruption layer between Safaricom's wire format and our contracts.
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
)

// CallbackToken format: 64 hex chars (see payment app ids.Secret(32)).
var tokenRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ValidToken(s string) bool { return tokenRe.MatchString(s) }

type STKResult struct {
	MerchantRequestID  string
	CheckoutRequestID  string
	ResultCode         int
	ResultDesc         string
	Amount             int64
	MpesaReceiptNumber string
	TransactionDate    string
	PhoneNumber        string
}

type wire struct {
	Body struct {
		STKCallback *struct {
			MerchantRequestID string      `json:"MerchantRequestID"`
			CheckoutRequestID string      `json:"CheckoutRequestID"`
			ResultCode        json.Number `json:"ResultCode"`
			ResultDesc        string      `json:"ResultDesc"`
			CallbackMetadata  struct {
				Item []struct {
					Name  string          `json:"Name"`
					Value json.RawMessage `json:"Value"`
				} `json:"Item"`
			} `json:"CallbackMetadata"`
		} `json:"stkCallback"`
	} `json:"Body"`
}

// Parse decodes a Daraja STK callback body.
func Parse(raw []byte) (STKResult, error) {
	var w wire
	if err := json.Unmarshal(raw, &w); err != nil {
		return STKResult{}, fmt.Errorf("invalid json: %w", err)
	}
	cb := w.Body.STKCallback
	if cb == nil || cb.CheckoutRequestID == "" {
		return STKResult{}, errors.New("missing Body.stkCallback.CheckoutRequestID")
	}
	code, err := strconv.Atoi(cb.ResultCode.String())
	if err != nil {
		return STKResult{}, fmt.Errorf("invalid ResultCode %q", cb.ResultCode)
	}
	r := STKResult{
		MerchantRequestID: cb.MerchantRequestID, CheckoutRequestID: cb.CheckoutRequestID,
		ResultCode: code, ResultDesc: cb.ResultDesc,
	}
	for _, it := range cb.CallbackMetadata.Item {
		v := scalar(it.Value)
		switch it.Name {
		case "Amount":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				r.Amount = int64(math.Round(f))
			}
		case "MpesaReceiptNumber":
			r.MpesaReceiptNumber = v
		case "TransactionDate":
			r.TransactionDate = v
		case "PhoneNumber":
			r.PhoneNumber = v
		}
	}
	return r, nil
}

// scalar renders a JSON string or number as text without float formatting
// (Daraja sends phone numbers and dates as large numbers).
func scalar(m json.RawMessage) string {
	var s string
	if json.Unmarshal(m, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(m, &n) == nil {
		return n.String()
	}
	return ""
}
