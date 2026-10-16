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
	{"303030", "Absa Bank Kenya"},
	{"329329", "Standard Chartered Bank"},
	{"222111", "Family Bank"},
	{"542542", "I&M Bank"},
	{"516600", "Diamond Trust Bank (DTB)"},
	{"972900", "Bank of Africa"},
	{"111777", "ABC Bank"},
	{"902363", "Bank of Baroda"},
	{"547700", "National Bank of Kenya"},
	{"552800", "SBM Bank"},
	{"700201", "Ecobank"},
	{"100400", "HF Group"},
	{"985050", "Gulf African Bank"},
	{"111999", "Sidian Bank"},
	{"982800", "Prime Bank"},
	{"559900", "UBA Kenya Bank"},
	{"972700", "Credit Bank"},
	{"508400", "Consolidated Bank"},
	{"862862", "Access Bank Kenya"},
	{"100229", "Citibank N.A. Kenya"},
	{"400222", "Development Bank of Kenya"},
	{"910200", "Guaranty Trust Bank (GTBank)"},
	{"344500", "Guardian Bank"},
	{"839900", "Middle East Bank"},
	{"986500", "M-Oriental Bank"},
	{"907950", "Paramount Bank"},
	{"919700", "Premier Bank Kenya"},
	{"842100", "Victoria Commercial Bank"},
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
