package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stripe/stripe-go/v81"
	stripepaymentintent "github.com/stripe/stripe-go/v81/paymentintent"
)

// fetchCardCountry returns the issuing country of the card attached to the
// successful PaymentIntent's latest charge. It deliberately does not fall back
// to a customer's saved payment methods because those may belong to a different
// payment and would misattribute the report's geography.
func fetchCardCountry(paymentIntentID string) (string, error) {
	paymentIntentID = strings.TrimSpace(paymentIntentID)
	if paymentIntentID == "" {
		return "", nil
	}
	key := strings.TrimSpace(setting.StripeApiSecret)
	if !strings.HasPrefix(key, "sk_") && !strings.HasPrefix(key, "rk_") {
		return "", errors.New("invalid Stripe API key")
	}
	stripe.Key = key

	params := &stripe.PaymentIntentParams{}
	params.AddExpand("latest_charge")
	paymentIntent, err := stripepaymentintent.Get(paymentIntentID, params)
	if err != nil {
		return "", err
	}
	if paymentIntent == nil || paymentIntent.LatestCharge == nil ||
		paymentIntent.LatestCharge.PaymentMethodDetails == nil ||
		paymentIntent.LatestCharge.PaymentMethodDetails.Card == nil {
		return "", nil
	}
	country := strings.ToUpper(strings.TrimSpace(paymentIntent.LatestCharge.PaymentMethodDetails.Card.Country))
	if len(country) != 2 {
		return "", nil
	}
	return country, nil
}
