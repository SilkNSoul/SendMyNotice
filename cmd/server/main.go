package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"errors"

	"sendmynotice/internal/apierrors"
	"sendmynotice/internal/mailer"
	"sendmynotice/internal/payment"
	"sendmynotice/internal/templates"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	mailer      *mailer.Client
	payment     *payment.Client
	squareAppID string
	squareLocID string
	squareJsURL string
}

func main() {
	// 1. Load Keys
	lobKey := os.Getenv("LOB_API_KEY")
	if lobKey == "" {
		log.Fatal("LOB_API_KEY not set")
	}

	squareToken := os.Getenv("SQUARE_ACCESS_TOKEN")
	if squareToken == "" {
		log.Fatal("SQUARE_ACCESS_TOKEN not set")
	}
	squareAppID := os.Getenv("SQUARE_APP_ID")
	if squareAppID == "" {
		log.Fatal("SQUARE_APP_ID not set")
	}
    squareLocID := os.Getenv("SQUARE_LOCATION_ID")
    if squareAppID == "" || squareLocID == "" {
        log.Fatal("SQUARE_APP_ID or SQUARE_LOCATION_ID not set")
    }

	appEnv := os.Getenv("APP_ENV")

	// 2. Configure Production vs Sandbox
	squareEnv := "sandbox"
	squareJsURL := "https://sandbox.web.squarecdn.com/v1/square.js"

	if appEnv == "production" {
		log.Println("🚨 STARTING IN PRODUCTION MODE")
		squareEnv = "production"
		squareJsURL = "https://web.squarecdn.com/v1/square.js" // Real JS URL
	} else {
		log.Println("⚠️  STARTING IN SANDBOX MODE")
	}
	// 3. Initialize Clients
	payClient := payment.NewClient(squareToken, squareEnv)

	srv := &Server{
		mailer:  mailer.NewClient(strings.TrimSpace(lobKey)),
		payment: payClient,
		squareAppID: squareAppID,
        squareLocID: squareLocID,
		squareJsURL: squareJsURL,
	}

	// 3. Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static Files (CSS, JS)
	workDir, _ := os.Getwd()
	filesDir := http.Dir(fmt.Sprintf("%s/web", workDir))
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(filesDir)))

	// Web Routes
	r.Get("/", srv.handleHome)
	r.Post("/web/preview", srv.handleWebPreview)
	
	// NEW: Merged Payment + Sending into one atomic action
	r.Post("/web/pay-and-send", srv.handlePayAndSend) 
	
	// (Optional) Keep lookup route if you ever un-hide the tool, 
	// but strictly for manual entry fallback logic.
	r.Post("/web/lookup-owner", srv.handleLookupOwner)

	log.Println("🚀 Server starting on :8080")
	http.ListenAndServe(":8080", r)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
    // Parse the index file as a template so we can inject the JS URL
    tmpl, err := template.ParseFiles("web/index.html")
    if err != nil {
        http.Error(w, "Could not load page", http.StatusInternalServerError)
        return
    }

    data := struct{ SquareJsURL string }{ SquareJsURL: s.squareJsURL }
    tmpl.Execute(w, data)
}

// handleWebPreview: Renders the confirmation modal
func (s *Server) handleWebPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Logic: If Job Site is blank, default to Owner Address
	jobSiteAddress := fmt.Sprintf("%s, %s, %s %s",
		r.FormValue("to_address1"),
		r.FormValue("to_city"),
		r.FormValue("to_state"),
		r.FormValue("to_zip"),
	)

	data := mailer.NoticeData{
		Date:           time.Now().Format("January 2, 2006"),
		SenderName:     r.FormValue("from_name"),
		SenderAddress:  fmt.Sprintf("%s, %s, %s %s", r.FormValue("from_address1"), r.FormValue("from_city"), r.FormValue("from_state"), r.FormValue("from_zip")),
		SenderRole:     r.FormValue("sender_role"),
		OwnerName:      r.FormValue("to_name"),
		OwnerAddress:   fmt.Sprintf("%s, %s, %s %s", r.FormValue("to_address1"), r.FormValue("to_city"), r.FormValue("to_state"), r.FormValue("to_zip")),
		JobSiteAddress: jobSiteAddress,
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

// RENDER THE MODAL WITH PAYMENT FORM
	fmt.Fprintf(w, `
		<div class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full flex items-center justify-center p-4 z-50">
			<div class="bg-white rounded-lg shadow-xl w-full max-w-2xl overflow-hidden">
				<div class="bg-gray-100 px-4 py-3 border-b flex justify-between items-center">
					<h3 class="font-bold text-lg">Review & Pay</h3>
					<button onclick="this.closest('.fixed').remove()" class="text-gray-500 hover:text-gray-700">&times;</button>
				</div>
				
				<div class="p-6 h-64 overflow-y-auto bg-gray-50 border-b relative">
					<div class="shadow-sm bg-white p-4 border mx-auto max-w-xl scale-90 origin-top">
						%s
					</div>
				</div>

				<div class="p-6 bg-white">
                    <div class="mb-4">
                        <label class="block text-sm font-medium text-gray-700 mb-2">Credit Card Details ($15.00)</label>
                        <div id="card-container" class="h-12"></div>
                    </div>

					<div class="flex justify-end gap-3">
						<button onclick="this.closest('.fixed').remove()" class="px-4 py-2 text-gray-600 hover:bg-gray-200 rounded">Edit</button>
						
                        <form id="payment-form" hx-post="/web/pay-and-send" hx-target="#result" hx-swap="innerHTML">
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

							<input type="hidden" id="square-token" name="square_token">

							<button type="button" id="card-button" class="bg-green-600 text-white font-bold py-2 px-6 rounded hover:bg-green-700 disabled:opacity-50">
								Pay & Send
							</button>
						</form>
					</div>
				</div>
			</div>
            
            <script>
                (async function() {
                    const payments = Square.payments('%s', '%s');
                    const card = await payments.card();
                    await card.attach('#card-container');

                    const cardButton = document.getElementById('card-button');
                    cardButton.addEventListener('click', async () => {
                        cardButton.disabled = true;
                        cardButton.innerText = "Processing...";
                        
                        try {
                            const result = await card.tokenize();
                            if (result.status === 'OK') {
                                document.getElementById('square-token').value = result.token;
                                // Trigger HTMX submission manually
                                htmx.trigger('#payment-form', 'submit');
                            } else {
                                alert(result.errors[0].message);
                                cardButton.disabled = false;
                                cardButton.innerText = "Pay & Send";
                            }
                        } catch (e) {
                            console.error(e);
                            cardButton.disabled = false;
                            cardButton.innerText = "Pay & Send";
                        }
                    });
                })();
            </script>
		</div>
	`,
		htmlBuffer.String(),
        // Hidden Inputs Args
		r.FormValue("to_name"), r.FormValue("to_address1"), r.FormValue("to_city"), r.FormValue("to_state"), r.FormValue("to_zip"),
		r.FormValue("from_name"), r.FormValue("from_address1"), r.FormValue("from_city"), r.FormValue("from_state"), r.FormValue("from_zip"),
		r.FormValue("job_description"), r.FormValue("estimated_price"), r.FormValue("sender_role"), r.FormValue("lender_name"),
		jobSiteAddress,
        // Square IDs for JS
        s.squareAppID, s.squareLocID,
	)
}

// handlePayAndSend: Charges card, THEN sends letter
func (s *Server) handlePayAndSend(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "<div class='text-red-500'>Error parsing form</div>", http.StatusBadRequest)
		return
	}

	// 1. PROCESS PAYMENT
	token := r.FormValue("square_token")
	if token == "" {
		fmt.Fprintf(w, `<div class="p-4 bg-red-100 text-red-700 border border-red-400 rounded">Error: Missing Payment Information</div>`)
		return
	}

	// Charge $15.00 (1500 cents)
	paymentID, err := s.payment.ChargeCard(r.Context(), token, 1500)
	if err != nil {
		log.Printf("Payment Error: %v", err)
		fmt.Fprintf(w, `<div class="p-4 bg-red-100 text-red-700 border border-red-400 rounded">Payment Declined: %s</div>`, err.Error())
		return
	}

	// 2. GENERATE & SEND LETTER (Only runs if payment succeeds)
	data := mailer.NoticeData{
		Date:           time.Now().Format("January 2, 2006"),
		SenderName:     r.FormValue("from_name"),
		SenderAddress:  fmt.Sprintf("%s, %s, %s %s", r.FormValue("from_address1"), r.FormValue("from_city"), r.FormValue("from_state"), r.FormValue("from_zip")),
		SenderRole:     r.FormValue("sender_role"),
		OwnerName:      r.FormValue("to_name"),
		OwnerAddress:   fmt.Sprintf("%s, %s, %s %s", r.FormValue("to_address1"), r.FormValue("to_city"), r.FormValue("to_state"), r.FormValue("to_zip")),
		JobSiteAddress: r.FormValue("job_site_address"),
		JobDescription: r.FormValue("job_description"),
		EstimatedPrice: r.FormValue("estimated_price"),
		LenderName:     r.FormValue("lender_name"),
	}

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

	// Create Request to Lob
	req := mailer.LetterRequest{
		Description: fmt.Sprintf("Notice - Ref: %s", paymentID), // Track Payment ID in Lob
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
		File:  htmlBuffer.String(),
	}

	resp, err := s.mailer.SendLetter(req)
	if err != nil {
		log.Printf("Mailer error: %v", err)
		var userErr *apierrors.UserError
		if errors.As(err, &userErr) {
			fmt.Fprintf(w, `<div class="p-4 bg-yellow-50 text-yellow-800 border border-yellow-400 rounded"><p class="font-bold">Check Address:</p><p>%s</p></div>`, userErr.UserMessage)
			return
		}
		// Critical: Payment succeeded but mail failed. In production, you'd alert yourself here.
		fmt.Fprintf(w, `<div class="p-4 bg-red-100 text-red-700 border border-red-400 rounded">System Error: Payment ID %s was successful, but letter generation failed. Please contact support.</div>`, paymentID)
		return
	}

	// 3. SUCCESS
	successHTML := fmt.Sprintf(`
		<div class="fixed inset-0 bg-gray-600 bg-opacity-50 flex items-center justify-center p-4">
			<div class="bg-white p-8 rounded shadow-xl text-center max-w-md">
				<div class="text-green-500 text-5xl mb-4">✓</div>
				<h3 class="text-gray-800 font-bold text-xl mb-2">Notice Sent!</h3>
				<p class="text-gray-600 mb-2">Lob ID: %s</p>
				<p class="text-xs text-gray-400 mb-4">Payment ID: %s</p>
				
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
	`, resp.ID, paymentID, resp.URL)

	w.Write([]byte(successHTML))
}

// handleLookupOwner: Still here if you ever un-hide the tool, but now purely for UI feedback
func (s *Server) handleLookupOwner(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error", http.StatusBadRequest)
		return
	}

	// We removed ATTOM, so we pass empty strings to force "Manual Entry" state
	s.renderOwnerFields(w,
		"", // No Name
		r.FormValue("to_address1"),
		r.FormValue("to_city"),
		r.FormValue("to_state"),
		r.FormValue("to_zip"),
		"Manual Entry Required", // Force Error Message
	)
}

func (s *Server) renderOwnerFields(w http.ResponseWriter, name, addr, city, state, zip, errorMsg string) {
	nameVal := name
	namePlaceholder := "Owner Name"
	statusBadge := ""
	inputBorder := "focus:ring-blue-500"
	bgClass := ""

	if errorMsg != "" {
		nameVal = ""
		namePlaceholder = "Owner not found - Please enter manually"
		statusBadge = fmt.Sprintf(`<span class="text-xs bg-yellow-100 text-yellow-800 px-2 py-0.5 rounded border border-yellow-200">⚠️ %s</span>`, errorMsg)
		inputBorder = "focus:ring-yellow-500 border-yellow-300"
		bgClass = "bg-yellow-50"
	}

	// UI FIX: Removed the duplicate "Property Owner" header text here
	fmt.Fprintf(w, `
		<div id="owner-fields" class="space-y-2">
            <div class="h-5 flex items-center mb-1">
               %s
            </div>
			
			<input type="text" name="to_name" value="%s" placeholder="%s" required 
				class="w-full border p-2 rounded outline-none focus:ring-2 %s %s">
			
			<input type="text" name="to_address1" value="%s" placeholder="Owner Address Line 1" required 
				class="w-full border p-2 rounded outline-none focus:ring-2 focus:ring-blue-500">
			
			<div class="grid grid-cols-2 gap-4">
				<input type="text" name="to_city" value="%s" placeholder="City" required 
					class="w-full border p-2 rounded outline-none focus:ring-2 focus:ring-blue-500">
				<div class="grid grid-cols-2 gap-2">
					<input type="text" name="to_state" value="%s" placeholder="State" required 
						class="w-full border p-2 rounded outline-none focus:ring-2 focus:ring-blue-500">
					<input type="text" name="to_zip" value="%s" placeholder="Zip" required 
						class="w-full border p-2 rounded outline-none focus:ring-2 focus:ring-blue-500">
				</div>
			</div>
		</div>
	`, statusBadge, nameVal, namePlaceholder, inputBorder, bgClass, addr, city, state, zip)
}