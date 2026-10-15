package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

// set -a; source .env; set +a
// go run .

var indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>M-Pesa STK Push</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 420px; margin: 60px auto; }
  label { display: block; margin-top: 12px; font-weight: 600; }
  input { width: 100%; padding: 8px; margin-top: 4px; box-sizing: border-box; }
  button { margin-top: 20px; padding: 10px 16px; width: 100%; }
  #result { margin-top: 20px; padding: 12px; border-radius: 6px; white-space: pre-wrap; }
  .ok { background: #e6ffed; border: 1px solid #34c759; }
  .err { background: #ffe6e6; border: 1px solid #ff3b30; }
</style>
</head>
<body>
  <h1>Send STK Push</h1>
  <form id="f">
    <label>Phone number</label>
    <input name="phone" placeholder="07XXXXXXXX" required>
    <label>Amount (KES)</label>
    <input name="amount" type="number" min="1" placeholder="100" required>

    <label>Payment destination</label>
    <label style="font-weight:normal;display:inline"><input type="radio" name="paymentType" value="bank" checked style="width:auto"> Bank</label>
    &nbsp;&nbsp;
    <label style="font-weight:normal;display:inline"><input type="radio" name="paymentType" value="paybill" style="width:auto"> Paybill</label>
    &nbsp;&nbsp;
    <label style="font-weight:normal;display:inline"><input type="radio" name="paymentType" value="till" style="width:auto"> Till</label>

    <div id="bankFields">
      <label>Destination bank</label>
      <select name="bankShortCode">
        <option value="" disabled selected>Select a bank</option>
        {{range .Banks}}<option value="{{.ShortCode}}">{{.Name}} ({{.ShortCode}})</option>
        {{end}}
      </select>
    </div>

    <div id="paybillFields" hidden>
      <label>Paybill number</label>
      <input name="paybillNumber" placeholder="e.g. 400200" maxlength="7">
    </div>

    <div id="accountNumberField">
      <label>Account number</label>
      <input name="accountNumber" placeholder="Account number" maxlength="12">
    </div>

    <div id="tillFields" hidden>
      <label>Till number</label>
      <input name="tillNumber" placeholder="e.g. 123456">
    </div>

    <button type="submit">Send</button>
  </form>
  <div id="result"></div>

<script>
const form = document.getElementById('f');
const result = document.getElementById('result');
const bankFields = document.getElementById('bankFields');
const paybillFields = document.getElementById('paybillFields');
const accountNumberField = document.getElementById('accountNumberField');
const tillFields = document.getElementById('tillFields');

function syncPaymentType() {
  const pt = form.paymentType.value; // bank | paybill | till
  bankFields.hidden = pt !== 'bank';
  paybillFields.hidden = pt !== 'paybill';
  accountNumberField.hidden = pt === 'till';
  tillFields.hidden = pt !== 'till';

  form.bankShortCode.required = pt === 'bank';
  form.paybillNumber.required = pt === 'paybill';
  form.accountNumber.required = pt === 'bank' || pt === 'paybill';
  form.tillNumber.required = pt === 'till';
}
form.paymentType.forEach(r => r.addEventListener('change', syncPaymentType));
syncPaymentType();

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  result.className = '';
  result.textContent = 'Sending...';
  const paymentType = form.paymentType.value;
  const data = {
    phone: form.phone.value,
    amount: form.amount.value,
    paymentType: paymentType,
    bankShortCode: paymentType === 'bank' ? form.bankShortCode.value : '',
    paybillNumber: paymentType === 'paybill' ? form.paybillNumber.value : '',
    tillNumber: paymentType === 'till' ? form.tillNumber.value : '',
    accountNumber: form.accountNumber.value,
  };
  try {
    const res = await fetch('/stkpush', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(data),
    });
    const body = await res.json();
    if (res.ok) {
      result.className = 'ok';
      result.textContent = body.CustomerMessage || JSON.stringify(body, null, 2);
    } else {
      result.className = 'err';
      result.textContent = body.error || JSON.stringify(body, null, 2);
    }
  } catch (err) {
    result.className = 'err';
    result.textContent = 'Request failed: ' + err;
  }
});
</script>
</body>
</html>`))

type stkPushInput struct {
	Phone         string `json:"phone"`
	Amount        string `json:"amount"`
	PaymentType   string `json:"paymentType"`
	BankShortCode string `json:"bankShortCode"`
	PaybillNumber string `json:"paybillNumber"`
	TillNumber    string `json:"tillNumber"`
	AccountNumber string `json:"accountNumber"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	client := NewMpesaClient(cfg)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		indexTmpl.Execute(w, map[string]any{"Banks": Banks})
	})

	mux.HandleFunc("POST /stkpush", func(w http.ResponseWriter, r *http.Request) {
		var in stkPushInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		amount, err := strconv.Atoi(in.Amount)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "amount must be a whole number")
			return
		}

		var paymentType PaymentType
		var destination string
		switch in.PaymentType {
		case string(PaymentTypeBank):
			paymentType, destination = PaymentTypeBank, in.BankShortCode
		case string(PaymentTypePaybill):
			paymentType, destination = PaymentTypePaybill, in.PaybillNumber
		case string(PaymentTypeTill):
			paymentType, destination = PaymentTypeTill, in.TillNumber
		default:
			writeJSONError(w, http.StatusBadRequest, "paymentType must be 'bank', 'paybill', or 'till'")
			return
		}

		result, err := client.InitiateSTKPush(in.Phone, amount, paymentType, destination, in.AccountNumber)
		if err != nil {
			log.Printf("stk push error: %v", err)
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}

		log.Printf("stk push sent: phone=%s amount=%d paymentType=%s destination=%s account=%s checkoutRequestID=%s",
			in.Phone, amount, paymentType, destination, in.AccountNumber, result.CheckoutRequestID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// M-Pesa calls this URL asynchronously to report whether the user
	// completed or cancelled the payment. Must be a public HTTPS URL
	// (e.g. via ngrok in development) matching MPESA_CALLBACK_URL.
	mux.HandleFunc("POST /mpesa/callback", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Printf("callback: failed to parse body: %v", err)
		} else {
			log.Printf("mpesa callback received: %+v", payload)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"ResultCode": "0", "ResultDesc": "Accepted"})
	})

	log.Printf("listening on %s (mpesa env base url: %s)", cfg.ListenAddr, cfg.BaseURL)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
