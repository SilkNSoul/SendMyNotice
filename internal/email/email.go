package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const resendEndpoint = "https://api.resend.com/emails"

type Client struct {
	apiKey string
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey}
}

type EmailRequest struct {
	From    string `json:"from"`
	To      []string `json:"to"`
	Subject string `json:"subject"`
	Html    string `json:"html"`
}

func (c *Client) Send(to, subject, htmlBody string) error {
	reqBody := EmailRequest{
		From:    "SendMyNotice <updates@sendmynotice.com>",
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil { return fmt.Errorf("marshalling email failed: %w", err) }

	req, err := http.NewRequest("POST", resendEndpoint, bytes.NewBuffer(jsonBytes))
	if err != nil { return fmt.Errorf("creating request failed: %w", err) }

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to send email: status %d", resp.StatusCode)
	}
	return nil
}