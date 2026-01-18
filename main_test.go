package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendLetter_Success(t *testing.T) {
	// 1. Setup Mock Server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate Method
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Validate Auth Header presence
		username, _, ok := r.BasicAuth()
		if !ok || username != "fake_key" {
			t.Error("Basic Auth header missing or incorrect")
		}

		// Mock Response from Lob
		response := LetterResponse{
			ID:          "ltr_fake123",
			ExpectedDel: "2025-01-01",
			URL:         "http://lob.com/preview.pdf",
		}
		w.WriteHeader(http.StatusOK)
		e := json.NewEncoder(w).Encode(response)
		if e != nil {
			log.Fatal(e)
		}
	}))
	defer mockServer.Close()

	// 2. Configure Client to use Mock Server
	client := NewLobClient("fake_key")
	client.BaseURL = mockServer.URL // Override URL to hit localhost

	// 3. Execute
	req := LetterRequest{
		Description: "Test",
		To:          Address{Name: "Test"},
		From:        Address{Name: "Test"},
		File:        "<html></html>",
	}

	resp, err := client.SendLetter(req)

	// 4. Assert
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if resp.ID != "ltr_fake123" {
		t.Errorf("Expected ID ltr_fake123, got %s", resp.ID)
	}
}

func TestSendLetter_APIError(t *testing.T) {
	// 1. Setup Mock Server for Failure
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		_, err := w.Write([]byte(`{"error": {"message": "Bad Key"}}`))
		if err != nil {
			log.Fatalf("Error writing data - %v", err)
		}

	}))
	defer mockServer.Close()

	client := NewLobClient("bad_key")
	client.BaseURL = mockServer.URL

	// 2. Execute
	req := LetterRequest{}
	_, err := client.SendLetter(req)

	// 3. Assert
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}
