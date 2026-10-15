package domain

// Bank is a bank payable via M-Pesa PayBill. Its Code is the bank's PayBill
// shortcode, used as PartyB; the customer's bank account number travels as
// AccountReference.
type Bank struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Banks is the catalog of destination banks (carried over from the legacy
// demo). Verify the shortcodes against current Safaricom/bank documentation
// before production use.
var Banks = []Bank{
	{"522522", "KCB Bank"},
	{"247247", "Equity Bank"},
	{"400200", "Co-operative Bank"},
	{"600100", "Stanbic Bank"},
	{"880100", "NCBA Bank"},
	{"972900", "Bank of Africa"},
	{"111777", "ABC Bank"},
	{"902363", "Bank of Baroda"},
}

var bankByCode = func() map[string]Bank {
	m := make(map[string]Bank, len(Banks))
	for _, b := range Banks {
		m[b.Code] = b
	}
	return m
}()

func BankByCode(code string) (Bank, bool) {
	b, ok := bankByCode[code]
	return b, ok
}
