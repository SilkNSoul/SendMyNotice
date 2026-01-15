package payment

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/square/square-go-sdk" // Root package now contains many models
	"github.com/square/square-go-sdk/client"
	"github.com/square/square-go-sdk/option" // Needed for authentication options
)

type Client struct {
	square *client.Client
}

// NewClient initializes the Square SDK with your access token
// env should be "sandbox" or "production"
func NewClient(accessToken, env string) *Client {
	// 1. Determine Environment
	// In the new SDK, environments are constants in the root package
	sqEnv := square.Environments.Sandbox
	if env == "production" {
		sqEnv = square.Environments.Production
	}

	// 2. Create Client using the 'option' package for config
	return &Client{
		square: client.NewClient(
			option.WithToken(accessToken),
			option.WithBaseURL(sqEnv),
		),
	}
}

// ChargeCard processes a payment using a sourceID (token) from the frontend
// amountCents: 1500 = $15.00
func (c *Client) ChargeCard(ctx context.Context, sourceID string, amountCents int64) (string, error) {
	// 1. Generate Idempotency Key
	idempotencyKey := uuid.New().String()

	// 2. Construct Request
	amount := &square.Money{
		Amount:   &amountCents,
		Currency: square.CurrencyUsd.Ptr(),
	}

	noteTemplate := "SendMyNotice Service Fee"

	req := &square.CreatePaymentRequest{
		SourceID:       sourceID,
		IdempotencyKey: idempotencyKey,
		AmountMoney:    amount,
		Note:           &noteTemplate,
	}

	// 3. Execute
	resp, err := c.square.Payments.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("square payment failed: %w", err)
	}

	// 4. Validate and Dereference
    // Check if Payment or ID is nil to avoid a panic
	if resp.Payment == nil || resp.Payment.ID == nil {
		return "", fmt.Errorf("payment succeeded but returned no payment ID")
	}

    // DEREFERENCE FIX: Use '*' to get the string value from the pointer
	paymentID := *resp.Payment.ID

	log.Printf("💰 Payment Successful! ID: %s", paymentID)
	return paymentID, nil
}

// RefundPayment refunds a payment if the letter generation fails
func (c *Client) RefundPayment(ctx context.Context, paymentID string) error {
    idempotencyKey := uuid.New().String()
    amountMoney := &square.Money{
        Amount:   nil, // Full refund if nil
        Currency: square.CurrencyUsd.Ptr(),
    }

    req := &square.RefundPaymentRequest{
        IdempotencyKey: idempotencyKey,
        PaymentID:      &paymentID,
        AmountMoney:    amountMoney,
        Reason:         func() *string { s := "System Error - Letter Not Sent"; return &s }(),
    }

    _, err := c.square.Refunds.RefundPayment(ctx, req)
    if err != nil {
        return fmt.Errorf("refund failed (CRITICAL - MANUALLY REFUND %s): %w", paymentID, err)
    }
    log.Printf("💸 Refunded Payment %s successfully", paymentID)
    return nil
}
