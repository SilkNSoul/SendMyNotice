package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"sendmynotice/internal/mailer" // Ensure this matches your go.mod module name

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

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
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