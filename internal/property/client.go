package property

import (
	"time"
)

// OwnerDetails holds the data we get back from the provider
type OwnerDetails struct {
	Name    string
	Address string // The mailing address of the owner (often different from property)
	City    string
	State   string
	Zip     string
}

// Client defines the behavior we need (Lookup)
type Client interface {
	LookupOwner(address, city, state, zip string) (*OwnerDetails, error)
}

// --- MOCK IMPLEMENTATION (For Development) ---

type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (m *MockClient) LookupOwner(address, city, state, zip string) (*OwnerDetails, error) {
	// Simulate API Latency
	time.Sleep(1 * time.Second)

	// Hardcoded logic for testing
	if address == "123 Main St" {
		return &OwnerDetails{
			Name:    "MOCK RICH INVESTOR LLC",
			Address: "999 Wall St",
			City:    "New York",
			State:   "NY",
			Zip:     "10005",
		}, nil
	}

	// Default fallback (echoes the input)
	return &OwnerDetails{
		Name:    "MOCK OWNER LOOKUP RESULT",
		Address: address,
		City:    city,
		State:   state,
		Zip:     zip,
	}, nil
}