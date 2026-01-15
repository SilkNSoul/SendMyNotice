package mailer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"sendmynotice/internal/apierrors"
)

const lobEndpoint = "https://api.lob.com/v1/letters"

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
	Description  string  `json:"description"`
	To           Address `json:"to"`
	From         Address `json:"from"`
	Color        bool    `json:"color"`
	File         string  `json:"file"`
	ExtraService string  `json:"extra_service,omitempty"` // <--- NEW
}

type LetterResponse struct {
	ID          string `json:"id"`
	ExpectedDel string `json:"expected_delivery_date"`
	URL         string `json:"url"`
	TrackingNumber string `json:"tracking_number"`
}

// LobErrorResponse matches the JSON structure Lob sends on failure
type LobErrorResponse struct {
	Error struct {
		Message    string `json:"message"`
		StatusCode int    `json:"status_code"`
		Code       string `json:"code"`
	} `json:"error"`
}

// NoticeData holds the specific fields for the Civil Code § 8200 form
type NoticeData struct {
	Date            string
	// The Contractor (User)
	SenderName      string
	SenderAddress   string
	SenderRole      string // e.g., "Subcontractor"
	
	// The Property Owner (Recipient)
	OwnerName       string
	OwnerAddress    string

	// Optional lender field
	LenderName     string

	// Project Details
	JobDescription  string
	JobSiteAddress  string
	EstimatedPrice  string
}
type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) SendLetter(l LetterRequest) (*LetterResponse, error) {
	jsonBytes, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("marshalling error: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, lobEndpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("request creation error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	
	// Basic Auth Manual Construction (Proven working)
	authString := c.apiKey + ":"
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(authString))
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var lobErr LobErrorResponse
		if jsonErr := json.Unmarshal(body, &lobErr); jsonErr == nil && lobErr.Error.Code != "" {
			// We successfully parsed a Lob error code
			return nil, apierrors.MapLobError(lobErr.Error.Code, lobErr.Error.Message)
		}
		
		// Fallback if the error body isn't JSON or doesn't match expected structure
		return nil, fmt.Errorf("api rejected request (status %d): %s", resp.StatusCode, string(body))
	}

	var result LetterResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("response decoding error: %w", err)
	}

	return &result, nil
}