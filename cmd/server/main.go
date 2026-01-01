package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"errors"
	"bytes"
	"time"
	"html/template"

	"sendmynotice/internal/mailer"
	"sendmynotice/internal/apierrors"
	"sendmynotice/internal/templates"

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
	r.Get("/", srv.handleHome)
	r.Post("/web/preview", srv.handleWebPreview) // NEW
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
	data := mailer.NoticeData{
		Date:           time.Now().Format("January 2, 2006"),
		SenderName:     r.FormValue("from_name"), // e.g. "SendMyNotice Inc"
		SenderAddress:  fmt.Sprintf("%s, %s, %s %s", r.FormValue("from_address1"), r.FormValue("from_city"), r.FormValue("from_state"), r.FormValue("from_zip")),
        SenderRole:     r.FormValue("sender_role"),
		OwnerName:      r.FormValue("to_name"),
		OwnerAddress:   fmt.Sprintf("%s, %s, %s %s", r.FormValue("to_address1"), r.FormValue("to_city"), r.FormValue("to_state"), r.FormValue("to_zip")),
		JobSiteAddress: r.FormValue("job_site_address"),
		JobDescription: r.FormValue("job_description"),
        EstimatedPrice: fmt.Sprintf("$%s", r.FormValue("estimated_price")),
		LenderName:     r.FormValue("lender_name"),
	}

	// 2. Execute Template into a Buffer
	tmpl, err := template.ParseFS(templates.GetNoticeFS(), "notice.html")	
	if err != nil {
		log.Printf("Template Parse Error: %v", err)
		http.Error(w, "System Error", http.StatusInternalServerError)
		return
	}

	var htmlBuffer bytes.Buffer
	if err := tmpl.Execute(&htmlBuffer, data); err != nil {
		log.Printf("Template Execute Error: %v", err)
		http.Error(w, "System Error", http.StatusInternalServerError)
		return
	}

	// 3. Create Request with the Rendered HTML
	req := mailer.LetterRequest{
		Description: "CA Prelim Notice",
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
		File:  htmlBuffer.String(), // INJECT THE RENDERED HTML HERE
	}

	// 3. Call Lob
	resp, err := s.mailer.SendLetter(req)
if err != nil {
		log.Printf("Mailer error: %v", err) // Log the technical error

		var userErr *apierrors.UserError
		
		// 1. Check if it's a known UserError
		if 	errors.As(err, &userErr) {
			// Render the FRIENDLY message
			fmt.Fprintf(w, `
				<div class="p-4 bg-yellow-50 text-yellow-800 border border-yellow-400 rounded">
					<p class="font-bold">Check Address:</p>
					<p>%s</p>
				</div>`, userErr.UserMessage)
			return
		}

		// 2. Generic System Error
		fmt.Fprintf(w, `<div class="p-4 bg-red-100 text-red-700 border border-red-400 rounded">System Error: Please try again later.</div>`)
		return
	}

	// 4. Return HTML Success Fragment
	successHTML := fmt.Sprintf(`
		<div class="fixed inset-0 bg-gray-600 bg-opacity-50 flex items-center justify-center p-4">
			<div class="bg-white p-8 rounded shadow-xl text-center max-w-md">
				<div class="text-green-500 text-5xl mb-4">✓</div>
				<h3 class="text-gray-800 font-bold text-xl mb-2">Letter Sent Successfully!</h3>
				<p class="text-gray-600 mb-4">Lob ID: %s</p>
				
				<div class="bg-blue-50 p-3 rounded text-sm text-blue-800 mb-4">
					<strong>Note:</strong> The PDF proof is being generated. It may take 10-15 seconds to appear.
				</div>

				<a href="%s" target="_blank" class="inline-block bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700">
					View PDF Proof
				</a>
				
				<button onclick="window.location.reload()" class="block mt-4 text-gray-500 text-sm hover:underline mx-auto">
					Send Another
				</button>
			</div>
		</div>
	`, resp.ID, resp.URL)

	w.Write([]byte(successHTML))
}

// handleWebPreview renders the HTML for user confirmation (No API call yet)
func (s *Server) handleWebPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// 1. Logic: If Job Site is blank (we don't have a field for it yet), default to Owner Address
	jobSiteAddress := fmt.Sprintf("%s, %s, %s %s", 
		r.FormValue("to_address1"), 
		r.FormValue("to_city"), 
		r.FormValue("to_state"), 
		r.FormValue("to_zip"),
	)

	// 2. Prepare Data for the PREVIEW (HTML Render)
	data := mailer.NoticeData{
		Date:           time.Now().Format("January 2, 2006"),
		SenderName:     r.FormValue("from_name"),
		SenderAddress:  fmt.Sprintf("%s, %s, %s %s", r.FormValue("from_address1"), r.FormValue("from_city"), r.FormValue("from_state"), r.FormValue("from_zip")),
		SenderRole:     r.FormValue("sender_role"),
		OwnerName:      r.FormValue("to_name"),
		OwnerAddress:   fmt.Sprintf("%s, %s, %s %s", r.FormValue("to_address1"), r.FormValue("to_city"), r.FormValue("to_state"), r.FormValue("to_zip")),
		JobSiteAddress: jobSiteAddress, // <--- Use our variable
		JobDescription: r.FormValue("job_description"),
		EstimatedPrice: r.FormValue("estimated_price"),
		LenderName:     r.FormValue("lender_name"),
	}

	tmpl, err := template.ParseFS(templates.GetNoticeFS(), "notice.html")
	if err != nil {
		log.Printf("Template Error: %v", err)
		http.Error(w, "System Error", http.StatusInternalServerError)
		return
	}
	var htmlBuffer bytes.Buffer
	if err := tmpl.Execute(&htmlBuffer, data); err != nil {
		log.Printf("Execute Error: %v", err)
		http.Error(w, "System Error", http.StatusInternalServerError)
		return
	}

	// 3. Return the Modal with HIDDEN inputs to pass data to the final handler
	fmt.Fprintf(w, `
		<div class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full flex items-center justify-center p-4">
			<div class="bg-white rounded-lg shadow-xl w-full max-w-2xl overflow-hidden">
				<div class="bg-gray-100 px-4 py-3 border-b flex justify-between items-center">
					<h3 class="font-bold text-lg">Review Your Letter</h3>
					<button onclick="this.closest('.fixed').remove()" class="text-gray-500 hover:text-gray-700">&times;</button>
				</div>
				
				<div class="p-6 h-96 overflow-y-auto bg-gray-50 border-b">
					<div class="shadow-sm bg-white p-4 border mx-auto max-w-xl scale-90 origin-top">
						%s
					</div>
				</div>

				<div class="px-4 py-3 bg-gray-50 flex justify-end gap-3">
					<button onclick="this.closest('.fixed').remove()" class="px-4 py-2 text-gray-600 hover:bg-gray-200 rounded">Edit</button>
					
					<form hx-post="/web/send" hx-target="#result" hx-swap="innerHTML">
						<input type="hidden" name="to_name" value="%s">
						<input type="hidden" name="to_address1" value="%s">
						<input type="hidden" name="to_city" value="%s">
						<input type="hidden" name="to_state" value="%s">
						<input type="hidden" name="to_zip" value="%s">
						
						<input type="hidden" name="from_name" value="%s">
						<input type="hidden" name="from_address1" value="%s">
						<input type="hidden" name="from_city" value="%s">
						<input type="hidden" name="from_state" value="%s">
						<input type="hidden" name="from_zip" value="%s">

						<input type="hidden" name="job_description" value="%s">
						<input type="hidden" name="estimated_price" value="%s">
						<input type="hidden" name="sender_role" value="%s">
						<input type="hidden" name="lender_name" value="%s">
						
						<input type="hidden" name="job_site_address" value="%s">

						<button type="submit" class="bg-green-600 text-white font-bold py-2 px-6 rounded hover:bg-green-700">
							Confirm & Send ($4.50)
						</button>
					</form>
				</div>
			</div>
		</div>
	`, 
	htmlBuffer.String(), 
	// Args for Hidden Inputs
	r.FormValue("to_name"), r.FormValue("to_address1"), r.FormValue("to_city"), r.FormValue("to_state"), r.FormValue("to_zip"),
	r.FormValue("from_name"), r.FormValue("from_address1"), r.FormValue("from_city"), r.FormValue("from_state"), r.FormValue("from_zip"),
	r.FormValue("job_description"), r.FormValue("estimated_price"), r.FormValue("sender_role"), r.FormValue("lender_name"),
	jobSiteAddress, // <--- IMPORTANT: The new argument
	)
}