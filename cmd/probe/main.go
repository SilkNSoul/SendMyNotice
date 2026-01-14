package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// CONFIG: Add the addresses you want to test here
var TestCases = []struct {
	Type    string
	Address string
	City    string
	State   string
}{
	// 1. YOUR PROBLEM ADDRESS (The Penthouse)
	{"User_Penthouse", "46 W Julian St #PH13", "San Jose", "CA"},
    // 2. YOUR BASE ADDRESS (The Building itself - might own the land)
	{"User_Building", "46 W Julian St", "San Jose", "CA"},
	// 3. COMMERCIAL (Google HQ - Owned by Corp)
	{"Commercial_Corp", "1600 Amphitheatre Pkwy", "Mountain View", "CA"},
	// 4. STANDARD SFH (Single Family Home - Owned by Person)
	{"Standard_House", "1098 Alta Ave", "Mountain View", "CA"},
    // 5. APARTMENT / UNIT (Random Unit - Check formatting)
    {"Apartment_Unit", "550 Moreland Way #1205", "Santa Clara", "CA"},
}

const (
	URL_Property   = "https://api.gateway.attomdata.com/propertyapi/v1.0.0/property/detail"
	URL_Assessment = "https://api.gateway.attomdata.com/propertyapi/v1.0.0/assessment/detail"
	URL_Sales      = "https://api.gateway.attomdata.com/propertyapi/v1.0.0/sale/detail"
)

func main() {
	apiKey := os.Getenv("ATTOM_API_KEY")
	if apiKey == "" {
		log.Fatal("❌ ATTOM_API_KEY is not set")
	}

	// Create a log file to save the massive JSON dumps
	f, err := os.Create("attom_debug_dump.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Multi-writer: log to console AND file
	mw := io.MultiWriter(os.Stdout, f)
	logger := log.New(mw, "", log.LstdFlags)

	client := &http.Client{Timeout: 15 * time.Second}

	logger.Println("========================================")
	logger.Println("   ATTOM API DIAGNOSTIC PROBE")
	logger.Println("========================================")

	for _, tc := range TestCases {
		logger.Printf("\n\n🔎 TESTING: [%s] %s, %s, %s", tc.Type, tc.Address, tc.City, tc.State)
		logger.Println(strings.Repeat("-", 50))

		// 1. Probe Assessment (Tax Record - Best for Owners)
		probeEndpoint(logger, client, apiKey, "ASSESSMENT", URL_Assessment, tc.Address, tc.City, tc.State)

		// 2. Probe Sales (Transaction Record - Best for Buyers/Recent Owners)
		probeEndpoint(logger, client, apiKey, "SALES", URL_Sales, tc.Address, tc.City, tc.State)

		// 3. Probe Property (General Info - Sometimes has Summary)
		probeEndpoint(logger, client, apiKey, "PROPERTY", URL_Property, tc.Address, tc.City, tc.State)
	}
    
    fmt.Println("\n✅ DONE. Full JSON dump saved to 'attom_debug_dump.txt'")
}

func probeEndpoint(logger *log.Logger, client *http.Client, key, label, endpoint, addr, city, state string) {
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("address1", addr)
	q.Set("address2", fmt.Sprintf("%s, %s", city, state))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("apikey", key)
	req.Header.Set("Accept", "application/json")

	logger.Printf("👉 Hitting %s...", label)
    
	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("   ❌ Network Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 1. Status Check
	if resp.StatusCode != 200 {
		logger.Printf("   ❌ API Error: Status %d\n", resp.StatusCode)
		return
	}

	// 2. Generic Map Parse (So we see ALL fields, not just structs)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Printf("   ❌ JSON Error: %v\n", err)
		return
	}

	// 3. Extract "property" array
	props, ok := result["property"].([]interface{})
	if !ok || len(props) == 0 {
		logger.Printf("   ⚠️  Success (200) but NO PROPERTY FOUND in array.\n")
		logger.Printf("   📝 Raw Response: %s\n", string(body))
		return
	}

	logger.Printf("   ✅ Found %d Record(s)\n", len(props))

	// 4. Inspect the first record deeply
	firstProp := props[0].(map[string]interface{})
    
    // Dump specific fields of interest based on Endpoint
	if label == "ASSESSMENT" {
		if assessment, ok := firstProp["assessment"].(map[string]interface{}); ok {
            if owner, ok := assessment["owner"].(map[string]interface{}); ok {
                logger.Printf("      [Assessment] Raw Owner Block: %+v\n", owner)
            } else {
                logger.Println("      [Assessment] 'owner' block missing!")
            }
		}
	} else if label == "SALES" {
        if sale, ok := firstProp["sale"].(map[string]interface{}); ok {
            if buyers, ok := sale["buyer"].([]interface{}); ok {
                 logger.Printf("      [Sales] Raw Buyers Block: %+v\n", buyers)
            }
        }
    }

	// 5. Dump Full JSON to file (compacted for readability line-by-line)
    // We print the first 500 chars to console so you get a taste
    jsonStr := string(body)
    snippet := jsonStr
    if len(snippet) > 300 { snippet = snippet[:300] + "..." }
	logger.Printf("      SNAPSHOT: %s\n", snippet)
    
    // Write full block to log file
    logger.Printf("      [FULL DUMP SAVED]\n")
}