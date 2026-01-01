package main

import (
"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"sendmynotice/internal/mailer"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	mailer *mailer.Client
}

func main() {
	rawKey := os.Getenv("LOB_API_KEY")
	if rawKey == "" {
		log.Fatal("LOB_API_KEY environment variable is not set")
	}
	apiKey := strings.TrimSpace(rawKey)

	srv := &Server{
		mailer: mailer.NewClient(apiKey),
	}

	r := chi.NewRouter()
	
	// Standard Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)

	// Routes
	r.Post("/api/send-test", srv.handleSendTest)
	r.Get("/", srv.handleHome)
	r.Post("/web/send", srv.handleWebSend)

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

// handleHome serves the static HTML file
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join("web", "index.html"))
}

// handleWebSend processes the form and returns an HTML Fragment
func (s *Server) handleWebSend(w http.ResponseWriter, r *http.Request) {
	// 1. Parse Form Data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "<div class='text-red-500'>Error parsing form</div>", http.StatusBadRequest)
		return
	}

	// 2. Map Form Fields to Mailer Struct
	req := mailer.LetterRequest{
		Description: "Web Form Submission",
		To: mailer.Address{
			Name:           r.FormValue("to_name"),
			AddressLine1:   r.FormValue("to_address1"),
			AddressCity:    r.FormValue("to_city"),
			AddressState:   r.FormValue("to_state"),
			AddressZip:     r.FormValue("to_zip"),
			AddressCountry: "US",
		},
		From: mailer.Address{
			Name:           r.FormValue("from_name"),
			AddressLine1:   r.FormValue("from_address1"),
			AddressCity:    r.FormValue("from_city"),
			AddressState:   r.FormValue("from_state"),
			AddressZip:     r.FormValue("from_zip"),
			AddressCountry: "US",
		},
		Color: false,
		File:  "<html><body><h1>Preliminary Notice</h1><p>Sent via Web Interface.</p></body></html>",
	}

	// 3. Call Lob
	resp, err := s.mailer.SendLetter(req)
	if err != nil {
		log.Printf("Mailer error: %v", err)
		// Return HTML error banner
		fmt.Fprintf(w, `<div class="p-4 bg-red-100 text-red-700 border border-red-400 rounded">Error: %v</div>`, err)
		return
	}

	// 4. Return HTML Success Fragment
	// We use Fprintf to construct a simple HTML response for HTMX to swap in.
	successHTML := fmt.Sprintf(`
		<div class="p-4 bg-green-50 border border-green-200 rounded text-center animate-pulse">
			<h3 class="text-green-800 font-bold text-lg">Letter Sent!</h3>
			<p class="text-sm text-green-600 mb-2">ID: %s</p>
			<a href="%s" target="_blank" class="inline-block bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700">
				View PDF Proof
			</a>
		</div>
	`, resp.ID, resp.URL)

	w.Write([]byte(successHTML))
}

// handleSendTest accepts a JSON payload and triggers the mailer
func (s *Server) handleSendTest(w http.ResponseWriter, r *http.Request) {
	var req mailer.LetterRequest
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := s.mailer.SendLetter(req)
	if err != nil {
		log.Printf("Mailer error: %v", err)
		http.Error(w, "Failed to send letter", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false) 
	if err := enc.Encode(resp); err != nil {
		log.Printf("Encoding error: %v", err)
	}
}