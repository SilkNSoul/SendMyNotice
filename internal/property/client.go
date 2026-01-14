package property

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OwnerDetails struct {
	Name    string
	Address string
	City    string
	State   string
	Zip     string
}

type Client interface {
	LookupOwner(address1, city, state, zip string) (*OwnerDetails, error)
}

type AttomClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewAttomClient(apiKey string) *AttomClient {
	return &AttomClient{
		apiKey: apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// ---------------------------------------------------------
// RESPONSE STRUCTS
// ---------------------------------------------------------

// Common status structure
type attomStatus struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type assessmentResponse struct {
	Status attomStatus `json:"status"`
	Property []struct {
		Assessment struct {
			Owner struct {
				Owner1 struct {
					FullName  string `json:"fullname"`
					LastName  string `json:"lastname"`
					FirstName string `json:"firstname"`
				} `json:"owner1"`
				CorpOwner struct {
					Name string `json:"name"`
				} `json:"corporation"`
			} `json:"owner"`
		} `json:"assessment"`
		Address struct {
			Line1      string `json:"line1"`
			Locality   string `json:"locality"`
			CountrySub string `json:"countrySubd"`
			Postal1    string `json:"postal1"`
		} `json:"address"`
	} `json:"property"`
}

type saleResponse struct {
	Status attomStatus `json:"status"`
	Property []struct {
		Sale struct {
			Amount struct {
				SaleAmt float64 `json:"saleamt"`
			} `json:"amount"`
			SaleTransDate string `json:"saletransdate"`
            // Sale endpoint lists Buyers
			Buyers []struct {
				FullName string `json:"fullname"`
			} `json:"buyer"`
		} `json:"sale"`
		Address struct {
			Line1      string `json:"line1"`
			Locality   string `json:"locality"`
			CountrySub string `json:"countrySubd"`
			Postal1    string `json:"postal1"`
		} `json:"address"`
	} `json:"property"`
}

// ---------------------------------------------------------
// LOGIC
// ---------------------------------------------------------

func (c *AttomClient) LookupOwner(address1, city, state, zip string) (*OwnerDetails, error) {
    // STRATEGY: Try Assessment first. If empty, try Sale.
    
    // 1. Try Assessment
    owner, err := c.fetchAssessment(address1, city, state)
    if err == nil {
        return owner, nil
    }
    
    log.Printf("⚠️ Assessment lookup failed (%v). Trying Sale records...", err)

    // 2. Fallback to Sale
    owner, err = c.fetchSale(address1, city, state)
    if err == nil {
        return owner, nil
    }

    return nil, fmt.Errorf("could not find owner in Assessment or Sale records")
}

func (c *AttomClient) fetchAssessment(address1, city, state string) (*OwnerDetails, error) {
	endpoint := "https://api.gateway.attomdata.com/propertyapi/v1.0.0/assessment/detail"
    u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("address1", address1)
	q.Set("address2", fmt.Sprintf("%s, %s", city, state))
	u.RawQuery = q.Encode()

    // Helper to perform request (omitted generic boilerplate for brevity, assuming you copy logic from previous step)
	bodyBytes, err := c.doRequest(u.String())
	if err != nil { return nil, err }

	var result assessmentResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil { return nil, err }

    if len(result.Property) == 0 { return nil, fmt.Errorf("no property found") }
    
    prop := result.Property[0]
    ownerStruct := prop.Assessment.Owner
	ownerName := ownerStruct.Owner1.FullName

	if ownerName == "" {
		if ownerStruct.Owner1.LastName != "" {
			ownerName = fmt.Sprintf("%s %s", ownerStruct.Owner1.FirstName, ownerStruct.Owner1.LastName)
		} else if ownerStruct.CorpOwner.Name != "" {
			ownerName = ownerStruct.CorpOwner.Name
		}
	}

    if strings.TrimSpace(ownerName) == "" { return nil, fmt.Errorf("empty name") }

    return &OwnerDetails{
		Name:    strings.ToUpper(strings.TrimSpace(ownerName)),
		Address: prop.Address.Line1,
		City:    prop.Address.Locality,
		State:   prop.Address.CountrySub,
		Zip:     prop.Address.Postal1,
	}, nil
}

func (c *AttomClient) fetchSale(address1, city, state string) (*OwnerDetails, error) {
	endpoint := "https://api.gateway.attomdata.com/propertyapi/v1.0.0/sale/detail"
    u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("address1", address1)
	q.Set("address2", fmt.Sprintf("%s, %s", city, state))
	u.RawQuery = q.Encode()

	bodyBytes, err := c.doRequest(u.String())
	if err != nil { return nil, err }

	var result saleResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil { return nil, err }

    if len(result.Property) == 0 { return nil, fmt.Errorf("no sale record found") }
    
    sale := result.Property[0].Sale
    if len(sale.Buyers) == 0 { return nil, fmt.Errorf("no buyers listed") }

    ownerName := sale.Buyers[0].FullName
    if ownerName == "" { return nil, fmt.Errorf("empty buyer name") }

    return &OwnerDetails{
		Name:    strings.ToUpper(strings.TrimSpace(ownerName)),
		Address: result.Property[0].Address.Line1,
		City:    result.Property[0].Address.Locality,
		State:   result.Property[0].Address.CountrySub,
		Zip:     result.Property[0].Address.Postal1,
	}, nil
}

func (c *AttomClient) doRequest(urlStr string) ([]byte, error) {
	req, _ := http.NewRequest(http.MethodGet, urlStr, nil)
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
    return io.ReadAll(resp.Body)
}

// ---------------------------------------------------------
// MOCK IMPLEMENTATION (Keep this for local dev)
// ---------------------------------------------------------

type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (m *MockClient) LookupOwner(address, city, state, zip string) (*OwnerDetails, error) {
	time.Sleep(500 * time.Millisecond)
	return &OwnerDetails{
		Name:    "MOCK ATTOM INVESTOR LLC",
		Address: "999 Wall St",
		City:    "New York",
		State:   "NY",
		Zip:     "10005",
	}, nil
}