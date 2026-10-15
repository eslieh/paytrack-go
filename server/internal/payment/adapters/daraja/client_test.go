package daraja

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/eslieh/paytrack-go/server/internal/payment/app"
	"github.com/eslieh/paytrack-go/server/internal/payment/domain"
)

func fakeDaraja(t *testing.T, stk http.HandlerFunc) (*Client, *atomic.Int32) {
	t.Helper()
	var oauthCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/v1/generate", func(w http.ResponseWriter, r *http.Request) {
		oauthCalls.Add(1)
		if u, p, _ := r.BasicAuth(); u != "ck" || p != "cs" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"access_token":"tok","expires_in":"3599"}`))
	})
	mux.HandleFunc("/mpesa/stkpush/v1/processrequest", stk)
	mux.HandleFunc("/mpesa/stkpushquery/v1/query", stk)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL, srv.URL), &oauthCalls
}

var ch = domain.Channel{Type: "till", Environment: "sandbox", ShortCode: "174379", TillNumber: "123456",
	ConsumerKey: "ck", ConsumerSecret: "cs", Passkey: "pk"}

func TestSTKPushTillAndTokenCache(t *testing.T) {
	c, oauth := fakeDaraja(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["TransactionType"] != "CustomerBuyGoodsOnline" || body["PartyB"] != "123456" || body["BusinessShortCode"] != "174379" {
			t.Errorf("unexpected body %v", body)
		}
		w.Write([]byte(`{"MerchantRequestID":"m","CheckoutRequestID":"ws_CO_1","ResponseCode":"0"}`))
	})
	for range 3 {
		ack, err := c.STKPush(context.Background(), ch, app.STKPushRequest{Amount: 1, Phone: "254712345678", AccountReference: "A", Description: "d", CallbackURL: "https://x"})
		if err != nil || ack.CheckoutRequestID != "ws_CO_1" {
			t.Fatalf("ack=%+v err=%v", ack, err)
		}
	}
	if n := oauth.Load(); n != 1 {
		t.Fatalf("oauth called %d times, want 1 (cached)", n)
	}
}

func TestSTKPushRejectedVsAmbiguous(t *testing.T) {
	c, _ := fakeDaraja(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"requestId":"1","errorCode":"400.002.02","errorMessage":"Bad Request - Invalid PartyB"}`))
	})
	_, err := c.STKPush(context.Background(), ch, app.STKPushRequest{Amount: 1})
	var rej *app.RejectedError
	if !errors.As(err, &rej) || rej.Code != "400.002.02" {
		t.Fatalf("want RejectedError, got %v", err)
	}

	c, _ = fakeDaraja(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	_, err = c.STKPush(context.Background(), ch, app.STKPushRequest{Amount: 1})
	if err == nil || errors.As(err, &rej) {
		t.Fatalf("want ambiguous error, got %v", err)
	}
}

func TestBadCredentialsRejected(t *testing.T) {
	c, _ := fakeDaraja(t, func(w http.ResponseWriter, r *http.Request) { t.Error("stk endpoint must not be reached") })
	bad := ch
	bad.ConsumerSecret = "wrong"
	_, err := c.STKPush(context.Background(), bad, app.STKPushRequest{Amount: 1})
	var rej *app.RejectedError
	if !errors.As(err, &rej) {
		t.Fatalf("want RejectedError, got %v", err)
	}
}

func TestSTKQuery(t *testing.T) {
	c, _ := fakeDaraja(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errorCode":"500.001.1001","errorMessage":"The transaction is being processed"}`))
	})
	q, err := c.STKQuery(context.Background(), ch, "ws_CO_1")
	if err != nil || !q.Pending {
		t.Fatalf("want pending, got %+v %v", q, err)
	}

	c, _ = fakeDaraja(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ResponseCode":"0","CheckoutRequestID":"ws_CO_1","ResultCode":"1032","ResultDesc":"Request cancelled by user"}`))
	})
	q, err = c.STKQuery(context.Background(), ch, "ws_CO_1")
	if err != nil || q.Pending || q.Result.Code != 1032 {
		t.Fatalf("got %+v %v", q, err)
	}
}
