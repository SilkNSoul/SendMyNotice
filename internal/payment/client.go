package payment

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/square/square-go-sdk"
	"github.com/square/square-go-sdk/client"
	"github.com/square/square-go-sdk/option"
)

type Client struct {
	square *client.Client
}

func NewClient(accessToken, env string) *Client {
	sqEnv := square.Environments.Sandbox
	if env == "production" {
		sqEnv = square.Environments.Production
	}

	return &Client{
		square: client.NewClient(
			option.WithToken(accessToken),
			option.WithBaseURL(sqEnv),
		),
	}
}

func (c *Client) ChargeCard(ctx context.Context, sourceID string, amountCents int64, userEmail string) (string, error) {
	idempotencyKey := uuid.New().String()

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
        BuyerEmailAddress: &userEmail, 
    }

	resp, err := c.square.Payments.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("square payment failed: %w", err)
	}

	if resp.Payment == nil || resp.Payment.ID == nil {
		return "", fmt.Errorf("payment succeeded but returned no payment ID")
	}

	paymentID := *resp.Payment.ID

	log.Printf("💰 Payment Successful! ID: %s", paymentID)
	return paymentID, nil
}

func (c *Client) RefundPayment(ctx context.Context, paymentID string, amountCents int64) error {
    idempotencyKey := uuid.New().String()
    
    amountMoney := &square.Money{
        Amount:   &amountCents, 
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
