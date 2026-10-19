package templates

// Samples is example data for every template, used by the preview endpoint
// and tests.
func Samples(appURL string) map[string]map[string]any {
	steps := []map[string]any{
		{"Label": "Verify your email", "Done": true, "URL": appURL + "/dashboard"},
		{"Label": "Add a channel (bank account, PayBill or Till)", "Done": true, "URL": appURL + "/dashboard/channels"},
		{"Label": "Create an API key for your backend", "Done": false, "URL": appURL + "/dashboard/api-keys"},
		{"Label": "Set your webhook URL", "Done": false, "URL": appURL + "/dashboard/webhooks"},
		{"Label": "Receive your first payment", "Done": false, "URL": appURL + "/dashboard/payments"},
	}
	device := map[string]any{"When": "16 Oct 2026, 09:41 EAT", "Device": "Chrome on macOS", "IP": "197.248.10.4"}
	with := func(m map[string]any) map[string]any {
		for k, v := range device {
			m[k] = v
		}
		return m
	}
	return map[string]map[string]any{
		"verify_email":      {"BusinessName": "Acme Shop", "VerifyURL": appURL + "/auth/verify-email?token=sample", "ExpiresHours": 24},
		"login_code":        with(map[string]any{"Code": "482913", "ExpiresMinutes": 10}),
		"password_reset":    {"ResetURL": appURL + "/auth/reset?token=sample", "ExpiresMinutes": 30},
		"password_changed":  with(map[string]any{}),
		"new_signin":        with(map[string]any{"Method": "your password and an email code"}),
		"two_step_disabled": with(map[string]any{}),
		"google_linked":     {"GoogleEmail": "owner@gmail.com", "When": "16 Oct 2026, 09:41 EAT"},
		"welcome":           {"BusinessName": "Acme Shop", "Steps": steps, "DashboardURL": appURL + "/dashboard"},
		"setup_reminder":    {"BusinessName": "Acme Shop", "Steps": steps, "DashboardURL": appURL + "/dashboard"},
		"first_payment":     {"BusinessName": "Acme Shop", "Amount": "KES 1,500", "ReceiptURL": appURL + "/receipt?id=pay_sample"},
		"payment_receipt": {
			"BusinessName": "Acme Shop", "Amount": "KES 1,500", "Receipt": "TJF7RT61SV", "Phone": "+254 7•• ••• 678",
			"Destination": "Equity Bank · 0123456789", "AccountReference": "0123456789", "Description": "Order #1042",
			"PaidAt": "16 Oct 2026, 09:41 EAT", "PaymentID": "pay_06gkx7v8vtmdtsw6vhna6h0sgw", "ReceiptURL": appURL + "/receipt?id=pay_sample",
		},
		"admin_login_code":  with(map[string]any{"Code": "730215", "ExpiresMinutes": 10}),
		"tenant_suspended":  {"BusinessName": "Acme Shop", "Reason": "Chargebacks under review", "DashboardURL": appURL + "/dashboard"},
		"tenant_reinstated": {"BusinessName": "Acme Shop", "DashboardURL": appURL + "/dashboard"},
		"account_disabled":  {"Reason": "Suspected account takeover"},
		"account_enabled":   {"SignInURL": appURL + "/auth"},
		"ticket_received": {"Ref": "#1042", "Subject": "Payment stuck in pending", "BusinessName": "Acme Shop",
			"Excerpt": "A customer paid KES 1,500 but the payment still shows pending.", "TicketURL": appURL + "/dashboard/support/tkt_sample"},
		"ticket_reply": {"Ref": "#1042", "Subject": "Payment stuck in pending", "BusinessName": "Acme Shop",
			"Excerpt": "Thanks for the details. Safaricom confirmed the payment and we've reconciled it…", "TicketURL": appURL + "/dashboard/support/tkt_sample"},
		"ticket_resolved": {"Ref": "#1042", "Subject": "Payment stuck in pending", "BusinessName": "Acme Shop", "Excerpt": "",
			"TicketURL": appURL + "/dashboard/support/tkt_sample#rate"},
		"support_staff_alert": {"Kind": "New ticket", "Ref": "#1042", "Subject": "Payment stuck in pending", "BusinessName": "Acme Shop",
			"RequesterEmail": "owner@acme.co.ke", "Excerpt": "A customer paid KES 1,500 but the payment still shows pending.",
			"TicketURL": appURL + "/admin/support/tkt_sample"},
		"webhook_failing": {
			"BusinessName": "Acme Shop", "WebhookURL": "https://acme.co.ke/hooks/paytrack", "EventType": "payment.succeeded",
			"PaymentID": "pay_06gkx7v8vtmdtsw6vhna6h0sgw", "Attempts": 11, "LastError": "endpoint returned HTTP 502",
			"DeliveriesURL": appURL + "/dashboard/webhooks",
		},
	}
}
