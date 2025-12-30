package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings" // Import added for TrimSpace
	"time"
)

// LobEndpoint is the production URL for letters.
const LobEndpoint = "https://api.lob.com/v1/letters"

type Address struct {
	Name           string `json:"name"`
	AddressLine1   string `json:"address_line1"`
	AddressLine2   string `json:"address_line2,omitempty"`
	AddressCity    string `json:"address_city"`
	AddressState   string `json:"address_state"`
	AddressZip     string `json:"address_zip"`
	AddressCountry string `json:"address_country"`
}

type LetterRequest struct {
	Description string  `json:"description"`
	To          Address `json:"to"`
	From        Address `json:"from"`
	Color       bool    `json:"color"`
	File        string  `json:"file"`
}

type LetterResponse struct {
	ID          string `json:"id"`
	ExpectedDel string `json:"expected_delivery_date"`
	URL         string `json:"url"`
}

type LobClient struct {
	APIKey     string
	HTTPClient *http.Client
	BaseURL    string
}

func NewLobClient(apiKey string) *LobClient {
	return &LobClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		BaseURL: LobEndpoint,
	}
}

func (c *LobClient) SendLetter(l LetterRequest) (*LetterResponse, error) {
	jsonBytes, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("marshalling error: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("request creation error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Manual Auth Construction
	// Lob requires "API_KEY:" encoded in base64.
	authString := c.APIKey + ":"
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(authString))
	req.Header.Set("Authorization", authHeader)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 200 OK or 201 Created are success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("api rejected request (status %d): %s", resp.StatusCode, string(body))
	}

	var result LetterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("response decoding error: %w", err)
	}

	return &result, nil
}

func main() {
	rawKey := os.Getenv("LOB_API_KEY")
	if rawKey == "" {
		log.Fatal("LOB_API_KEY environment variable is not set")
	}

	// TRIM WHITESPACE: This fixes the 401 if a newline slipped in
	apiKey := strings.TrimSpace(rawKey)

	client := NewLobClient(apiKey)

	// Using the VALID addresses from your successful Curl
	reqBody := LetterRequest{
		Description: "SendMyNotice Go Test",
		To: Address{
			Name:           "Harry Homeowner",
			AddressLine1:   "46 W Julian St",
			AddressCity:    "San Jose",
			AddressState:   "CA",
			AddressZip:     "95110",
			AddressCountry: "US",
		},
		From: Address{
			Name:           "Me",
			AddressLine1:   "55 Devine St",
			AddressCity:    "San Jose",
			AddressState:   "CA",
			AddressZip:     "95110",
			AddressCountry: "US",
		},
		Color: false,
		File:  `<html><body><h1>Preliminary Notice</h1><p>MVP Connection Test.</p></body></html>`,
	}

	resp, err := client.SendLetter(reqBody)
	if err != nil {
		log.Fatalf("Fatal Error: %v", err)
	}

	fmt.Println("--- Letter Sent Successfully ---")
	fmt.Printf("Letter ID: %s\n", resp.ID)
	fmt.Printf("Preview URL: %s\n", resp.URL)
}
