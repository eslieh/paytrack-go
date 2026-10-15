package domain

import "testing"

func TestParseSuccess(t *testing.T) {
	raw := []byte(`{"Body":{"stkCallback":{"MerchantRequestID":"29115-34620561-1","CheckoutRequestID":"ws_CO_191220191020363925",
	"ResultCode":0,"ResultDesc":"The service request is processed successfully.","CallbackMetadata":{"Item":[
	{"Name":"Amount","Value":1.00},{"Name":"MpesaReceiptNumber","Value":"NLJ7RT61SV"},{"Name":"Balance"},
	{"Name":"TransactionDate","Value":20191219102115},{"Name":"PhoneNumber","Value":254708374149}]}}}}`)
	r, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if r.ResultCode != 0 || r.Amount != 1 || r.MpesaReceiptNumber != "NLJ7RT61SV" ||
		r.PhoneNumber != "254708374149" || r.TransactionDate != "20191219102115" {
		t.Fatalf("got %+v", r)
	}
}

func TestParseCancelled(t *testing.T) {
	r, err := Parse([]byte(`{"Body":{"stkCallback":{"MerchantRequestID":"m","CheckoutRequestID":"c","ResultCode":1032,"ResultDesc":"Request cancelled by user"}}}`))
	if err != nil || r.ResultCode != 1032 {
		t.Fatalf("got %+v %v", r, err)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, raw := range []string{`not json`, `{}`, `{"Body":{"stkCallback":{"CheckoutRequestID":"c","ResultCode":"x"}}}`} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("Parse(%s) succeeded", raw)
		}
	}
}
