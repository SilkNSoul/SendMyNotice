package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"sendmynotice/internal/apierrors"
	"sendmynotice/internal/mailer"
	"sendmynotice/internal/payment"
	"sendmynotice/internal/templates"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var fileMutex sync.Mutex

type DiscordMessage struct {
	Content string `json:"content"`
}

type PageData struct {
    SquareJsURL string
    CurrentDate string
}

type OrderRecord struct {
    Time           string `json:"time"`
    PaymentID      string `json:"payment_id"`
    TrackingNumber string `json:"tracking_number"`
    Amount         string `json:"amount"`
    UserEmail      string `json:"user_email"`
    JobAddress     string `json:"job_address"`
}

type Server struct {
	mailer      *mailer.Client
	payment     *payment.Client
	squareAppID string
	squareLocID string
	squareJsURL string
	homeTemplate *template.Template
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

	tmpl, err := template.ParseFiles("web/index.html")
    if err != nil {
        log.Fatal("Failed to parse index.html: ", err)
    }

	srv := &Server{
		mailer:      mailer.NewClient(strings.TrimSpace(lobKey)),
		payment:     payClient,
		squareAppID: squareAppID,
		squareLocID: squareLocID,
		squareJsURL: squareJsURL,
		homeTemplate: tmpl,
	}

	// 3. Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static Files (CSS, JS)
	workDir, _ := os.Getwd()
	filesDir := http.Dir(fmt.Sprintf("%s/web", workDir))
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(filesDir)))

	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "favicon.ico")
	})

	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "User-agent: *\nAllow: /")
	})
	
	r.Get("/", srv.handleHome)

	r.Post("/web/preview", srv.handleWebPreview)

	// NEW: Merged Payment + Sending into one atomic action
	r.Post("/web/pay-and-send", srv.handlePayAndSend)

	// (Optional) Keep lookup route if you ever un-hide the tool,
	// but strictly for manual entry fallback logic.
	r.Post("/web/lookup-owner", srv.handleLookupOwner)

	r.Get("/web/check-pdf", srv.handleCheckPDFStatus)

	r.Post("/web/capture-lead", srv.handleCaptureLead)

	// HARDENING: Custom Server Config
    srvObj := &http.Server{
        Addr:         ":8080",
        Handler:      r,
        ReadTimeout:  15 * time.Second,  // Prevent Slowloris
        WriteTimeout: 15 * time.Second,  // Prevent stale connections
        IdleTimeout:  60 * time.Second,
    }

    log.Println("🚀 Server starting on :8080 (Production Config)")
    if err := srvObj.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server startup failed: %v", err)
    }
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
    data := PageData{
        SquareJsURL: s.squareJsURL,
        CurrentDate: time.Now().Format("Jan 02, 2006"),
    }
    // [FIX] Execute the cached template
    if err := s.homeTemplate.Execute(w, data); err != nil {
        log.Printf("Template execution failed: %v", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    }
}

func (s *Server) handleWebPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Logic: If Job Site is blank, default to Owner Address
	jobSiteAddress := r.FormValue("job_site_address")
	if jobSiteAddress == "" {
		jobSiteAddress = fmt.Sprintf("%s, %s, %s %s",
			r.FormValue("to_address1"),
			r.FormValue("to_city"),
			r.FormValue("to_state"),
			r.FormValue("to_zip"),
		)
	}

	// Prepare data for the Modal
	modalData := struct {
		NoticeHTML  template.HTML
		ToName      string
		ToAddress   string
		FromName    string
		SenderRole  string
		SquareAppID string
		SquareLocID string
		// DATA PRESERVATION: We pass the raw values to hidden inputs
		HiddenInputs map[string]string
	}{
		ToName:      r.FormValue("to_name"),
		ToAddress:   r.FormValue("to_address1"),
		FromName:    r.FormValue("from_name"),
		SenderRole:  r.FormValue("sender_role"),
		SquareAppID: s.squareAppID,
		SquareLocID: s.squareLocID,
		HiddenInputs: map[string]string{
			"to_name":          r.FormValue("to_name"),
			"to_address1":      r.FormValue("to_address1"),
			"to_city":          r.FormValue("to_city"),
			"to_state":         r.FormValue("to_state"),
			"to_zip":           r.FormValue("to_zip"),
			"from_name":        r.FormValue("from_name"),
			"from_address1":    r.FormValue("from_address1"),
			"from_city":        r.FormValue("from_city"),
			"from_state":       r.FormValue("from_state"),
			"from_zip":         r.FormValue("from_zip"),
			"sender_role":      r.FormValue("sender_role"),
			"job_description":  r.FormValue("job_description"),
			"estimated_price":  r.FormValue("estimated_price"),
			"lender_name":      r.FormValue("lender_name"),
			"job_site_address": jobSiteAddress,
			"user_email":       r.FormValue("user_email"),
		},
	}

	go s.sendLeadToDiscord(r.FormValue("user_email"), r.FormValue("from_name"), "LEAD_PREVIEW_GENERATED")

	// Render the Inner Notice HTML (Visual Preview)
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

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")

	noticeTmpl, _ := template.ParseFS(templates.GetNoticeFS(), "notice.html")
	var noticeBuff bytes.Buffer
	noticeTmpl.Execute(&noticeBuff, data)
	modalData.NoticeHTML = template.HTML(noticeBuff.String())

	const modalTemplate = `
	<div class="fixed inset-0 z-50 overflow-y-auto" aria-labelledby="modal-title" role="dialog" aria-modal="true">
		<div class="flex items-end justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:block sm:p-0">
			<div class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity" onclick="document.getElementById('result').innerHTML=''"></div>

			<div class="inline-block align-bottom bg-white rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-lg sm:w-full">
				<div class="bg-white px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
					<div class="sm:flex sm:items-start">
						<div class="mt-3 text-center sm:mt-0 sm:text-left w-full">
							<div class="flex justify-between items-center mb-4">
								<h3 class="text-lg leading-6 font-bold text-gray-900" id="modal-title">Confirm & Send</h3>
								<button onclick="document.getElementById('result').innerHTML=''" class="text-gray-400 hover:text-gray-500">
									<svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
								</button>
							</div>

							<div class="border border-gray-200 rounded-md bg-gray-50 p-4 mb-6 max-h-60 overflow-y-auto shadow-inner text-[10px] leading-relaxed">
								{{.NoticeHTML}}
							</div>

							<div class="bg-blue-50 p-4 rounded-md border border-blue-100">
								<div class="flex justify-between items-center mb-3">
									<span class="font-bold text-blue-900">Total</span>
									<span class="font-bold text-blue-900 text-xl">$29.00</span>
								</div>
								
								<div id="card-container" class="min-h-[50px] mb-4 bg-white rounded p-1"></div>

								<form id="payment-form" hx-post="/web/pay-and-send" hx-target="#result" hx-swap="innerHTML">
									{{range $key, $value := .HiddenInputs}}
										<input type="hidden" name="{{$key}}" value="{{$value}}">
									{{end}}
									<input type="hidden" name="square_token" id="square_token_input">
									
									<div class="mb-4 flex items-start">
										<div class="flex items-center h-5">
											<input id="tos_agree" name="tos_agree" type="checkbox" required 
												class="focus:ring-blue-500 h-4 w-4 text-blue-600 border-gray-300 rounded"
												onchange="document.getElementById('card-button').disabled = !this.checked; document.getElementById('card-button').classList.toggle('opacity-50', !this.checked)">
										</div>
										<div class="ml-2 text-xs text-gray-600 text-left">
											I agree to the <button type="button" onclick="document.getElementById('tos-modal').classList.remove('hidden')" class="text-blue-600 underline">Terms of Service</button> and understand that SendMyNotice is a filing service, not a law firm.
										</div>
									</div>

									<div class="bg-yellow-50 border border-yellow-100 p-2 rounded mb-4 flex items-start gap-2">
										<svg class="w-4 h-4 text-yellow-600 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
										<div class="text-[10px] text-yellow-800 text-left">
											<strong>Deadline Warning:</strong> If you started work more than 20 days ago, you must send this TODAY to protect your rights. USPS pickup is at 4:00 PM.
										</div>
									</div>

									<button type="button" id="card-button" disabled class="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-3 bg-green-600 text-base font-medium text-white hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500 sm:text-sm transition opacity-50 cursor-not-allowed">
										Pay & Send via Certified Mail
									</button>

									<div class="mt-4 flex items-center justify-center gap-3 bg-gray-50 p-2 rounded border border-gray-100">
										<div class="flex items-center gap-1">
											<svg class="w-4 h-4 text-gray-500" fill="currentColor" viewBox="0 0 24 24"><path d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zm-6 9c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zm3.1-9H8.9V6c0-1.71 1.39-3.1 3.1-3.1 1.71 0 3.1 1.39 3.1 3.1v2z"/></svg>
											<span class="text-[10px] font-bold text-gray-500 uppercase tracking-wide">256-Bit SSL Encrypted</span>
										</div>
										<div class="h-3 w-px bg-gray-300"></div>
										<div class="flex items-center gap-1">
											<svg class="w-4 h-4 text-gray-500" fill="currentColor" viewBox="0 0 24 24"><path d="M20 4H4c-1.11 0-1.99.89-1.99 2L2 18c0 1.11.89 2 2 2h16c1.11 0 2-.89 2-2V6c0-1.11-.89-2-2-2zm0 14H4v-6h16v6zm0-10H4V6h16v2z"/></svg>
											<span class="text-[10px] font-bold text-gray-500 uppercase tracking-wide">Secure Payment</span>
										</div>
									</div>
									<p class="text-[9px] text-gray-400 text-center mt-2">
										We do not store your credit card details. Payments are processed securely by Square®.
									</p>

								</form>
								<div id="payment-status-container" class="mt-2 text-center text-xs text-red-600 font-bold min-h-[20px]"></div>
							</div>
						</div>
					</div>
				</div>
				<div class="bg-gray-50 px-4 py-3 sm:px-6 flex justify-center">
					<form hx-post="/web/capture-lead" hx-swap="none">
						<input type="hidden" name="email" value="{{index .HiddenInputs "user_email"}}">
						<input type="hidden" name="from_name" value="{{.FromName}}">
						<input type="hidden" name="sender_role" value="{{.SenderRole}}">
						<button type="submit" class="text-xs text-gray-400 hover:text-gray-600 underline">
							No thanks, I'll print it myself
						</button>
					</form>
				</div>
			</div>
		</div>

		<script>
			async function initializeCard(appId, locationId) {
				if (!window.Square) { 
					console.error("Square JS not loaded");
					return;
				}
				
				try {
					const payments = Square.payments(appId, locationId);
					const card = await payments.card();
					await card.attach('#card-container');

					document.getElementById('card-button').addEventListener('click', async () => {
						const statusContainer = document.getElementById('payment-status-container');
						const btn = document.getElementById('card-button');
						
						// Disable button to prevent double charge
						btn.disabled = true;
						btn.innerText = "Processing...";
						statusContainer.innerText = "";
						
						try {
							const result = await card.tokenize();
							if (result.status === 'OK') {
								// Inject token into hidden field inside the form
								document.getElementById('square_token_input').value = result.token;
								// Trigger HTMX manually on the form
								htmx.trigger('#payment-form', 'submit');
							} else {
								statusContainer.innerText = result.errors[0].message;
								btn.disabled = false;
								btn.innerText = "Pay & Send via Certified Mail";
							}
						} catch (e) {
							console.error(e);
							statusContainer.innerText = "Payment System Error. Try again.";
							btn.disabled = false;
							btn.innerText = "Pay & Send via Certified Mail";
						}
					});
				} catch (e) {
					console.error("Square Init Error:", e);
				}
			}
			initializeCard('{{.SquareAppID}}', '{{.SquareLocID}}');
		</script>
	</div>
	`

	t, _ := template.New("modal").Parse(modalTemplate)
	t.Execute(w, modalData)
}

func (s *Server) handlePayAndSend(w http.ResponseWriter, r *http.Request) {
	const amountToCharge int64 = 2900

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

	userEmail := r.FormValue("user_email")

	finalJobSite := r.FormValue("job_site_address")
	if finalJobSite == "" {
		// If they checked "Same as Owner", this field is empty, so we construct it from Owner Addr
		finalJobSite = fmt.Sprintf("%s, %s, %s %s", 
			r.FormValue("to_address1"), r.FormValue("to_city"), r.FormValue("to_state"), r.FormValue("to_zip"))
	}

	// 2. GENERATE & SEND LETTER (Only runs if payment succeeds)
	data := mailer.NoticeData{
		Date:           time.Now().Format("January 2, 2006"),
		SenderName:     r.FormValue("from_name"),
		SenderAddress:  fmt.Sprintf("%s, %s, %s %s", r.FormValue("from_address1"), r.FormValue("from_city"), r.FormValue("from_state"), r.FormValue("from_zip")),
		SenderRole:     r.FormValue("sender_role"),
		OwnerName:      r.FormValue("to_name"),
		OwnerAddress:   fmt.Sprintf("%s, %s, %s %s", r.FormValue("to_address1"), r.FormValue("to_city"), r.FormValue("to_state"), r.FormValue("to_zip")),
		JobSiteAddress: finalJobSite,
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

	// Charge $29.00 (2900 cents)
	paymentID, err := s.payment.ChargeCard(r.Context(), token, amountToCharge, userEmail)
	if err != nil {
		log.Printf("Payment Error: %v", err)
		fmt.Fprintf(w, `<div class="p-4 bg-red-100 text-red-700 border border-red-400 rounded">Payment Declined: %s</div>`, err.Error())
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
		Color:        false,
		File:         htmlBuffer.String(),
		ExtraService: "certified", // This triggers the Tracking Number
	}

	resp, err := s.mailer.SendLetter(req)
	if err != nil {
		log.Printf("Mailer error: %v", err)

		refundErr := s.payment.RefundPayment(r.Context(), paymentID, amountToCharge)
		refundMsg := "Your card was refunded automatically."
		if refundErr != nil {
			// In a real app, pageer duty triggers here.
			// For MVP, we log loud and tell user to contact support.
			log.Printf("CRITICAL: FAILED TO REFUND %s: %v", paymentID, refundErr)
			go s.sendLeadToDiscord(userEmail, "SYSTEM_CRITICAL_FAILURE", fmt.Sprintf("CHARGE WITHOUT SERVICE! REFUND FAILED. PaymentID: %s", paymentID))

			refundMsg = fmt.Sprintf("Refund failed. Please contact support with Ref: %s", paymentID)
		}

		var userErr *apierrors.UserError
		if errors.As(err, &userErr) {
			fmt.Fprintf(w, `<div class="p-4 bg-yellow-50 text-yellow-800 border border-yellow-400 rounded"><p class="font-bold">Address Error:</p><p>%s</p><p class="text-sm mt-2 font-bold">%s</p></div>`, userErr.UserMessage, refundMsg)
			return
		}

		fmt.Fprintf(w, `<div class="p-4 bg-red-100 text-red-700 border border-red-400 rounded">System Error: Letter generation failed. %s</div>`, refundMsg)
		return
	}

	// 3. SUCCESS HTML
	go s.sendLeadToDiscord(userEmail, r.FormValue("from_name"), "PAID_CUSTOMER_$$$")
	
	encodedURL := url.QueryEscape(resp.URL)
	trackingLink := fmt.Sprintf("https://tools.usps.com/go/TrackConfirmAction?tLabels=%s", resp.TrackingNumber)

	successHTML := fmt.Sprintf(`
        <div class="fixed inset-0 bg-gray-600 bg-opacity-50 flex items-center justify-center p-4 z-50">
            <div class="bg-white rounded-lg shadow-xl max-w-md w-full animate-fade-in-up overflow-hidden">
                <div class="bg-green-50 p-6 text-center border-b border-green-100">
                    <div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-green-100 mb-3">
                        <svg class="h-6 w-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>
                    </div>
                    <h3 class="text-lg font-bold text-gray-900">Notice Sent Successfully!</h3>
                    <p class="text-sm text-gray-500 mt-1">Ref: %s</p>
                </div>

                <div class="p-6 space-y-5">
                    <div class="bg-white border rounded-lg p-3 shadow-sm">
                        <p class="text-xs text-gray-500 uppercase tracking-wide font-semibold mb-1">USPS Certified Mail®</p>
                        <div class="flex items-center justify-between">
                            <span class="text-lg font-mono font-bold text-gray-800 select-all">%s</span>
                            <a href="%s" target="_blank" class="text-blue-600 hover:text-blue-800 text-sm font-semibold flex items-center gap-1">
                                Track <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"></path></svg>
                            </a>
                        </div>
                    </div>

                    <div hx-get="/web/check-pdf?url=%s" hx-trigger="load" hx-swap="outerHTML">
                        <div class="block w-full bg-gray-50 text-gray-400 px-4 py-3 rounded text-center border border-dashed border-gray-300 text-sm">
                            <span class="inline-block animate-pulse">⏳ Generating PDF Proof...</span>
                        </div>
                    </div>

                    <div class="pt-4 border-t">
                        <p class="text-sm font-medium text-gray-700 mb-2 text-center">Know another contractor?</p>
                        <button onclick="navigator.clipboard.writeText('https://sendmynotice.com'); this.innerText = 'Link Copied!'" 
                                class="w-full flex items-center justify-center gap-2 bg-indigo-50 text-indigo-700 px-4 py-2 rounded border border-indigo-100 hover:bg-indigo-100 transition text-sm font-medium">
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z"></path></svg>
                            Copy Link to Share
                        </button>
                    </div>

                    <button onclick="window.location.reload()" class="block w-full text-center text-gray-400 text-xs hover:text-gray-600 hover:underline">
                        Start New Notice
                    </button>
                </div>
            </div>
        </div>
    `,
		paymentID,           // Ref
		resp.TrackingNumber, // Tracking Display
		trackingLink,        // Tracking Href
		encodedURL,          // PDF Poller
	)

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

func (s *Server) handleCheckPDFStatus(w http.ResponseWriter, r *http.Request) {
	pdfURL := r.URL.Query().Get("url") // This decodes the %26 back to & automatically
	if pdfURL == "" {
		return
	}

	// SECURITY CHECK: Only allow Lob URLs
	// Assuming Lob URLs look like "https://lob-assets.com/..."
	if !strings.HasPrefix(pdfURL, "https://") || !strings.Contains(pdfURL, "lob") {
		http.Error(w, "Invalid URL", http.StatusForbidden)
		return
	}

	// 1. Check if the PDF exists (Using the decoded, valid URL)
	resp, err := http.Head(pdfURL)

	// 2. LOGIC: If it's NOT ready, keep polling
	if err != nil || resp.StatusCode != http.StatusOK {

		encodedURL := url.QueryEscape(pdfURL)

		fmt.Fprintf(w, `
			<div hx-get="/web/check-pdf?url=%s" 
				hx-trigger="load delay:1s" 
				hx-swap="outerHTML" 
				class="flex flex-col items-center justify-center w-full bg-gray-50 text-blue-600 px-6 py-6 rounded border border-gray-200">
				<svg class="animate-spin h-8 w-8 text-blue-600 mb-3" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<span class="text-sm font-semibold text-gray-600">Encrypting & Finalizing Legal Document...</span>
				<span class="text-xs text-gray-400 mt-1">This ensures legal compliance.</span>
			</div>
		`, encodedURL)
		return
	}

	// 3. SUCCESS
	fmt.Fprintf(w, `
		<a href="%s" target="_blank" class="block w-full bg-blue-600 text-white px-6 py-3 rounded hover:bg-blue-700 transition text-center shadow-md font-bold">
			View PDF Proof
		</a>
	`, pdfURL) 
}


func (s *Server) logLead(email, name, role string) {
    // structured log entry
    entry := map[string]string{
        "event":     "LEAD_CAPTURE", // Easy to grep in Railway logs
        "timestamp": time.Now().Format(time.RFC3339),
        "email":     email,
        "name":      name,
        "role":      role,
        "status":    "FREE_TIER_DOWNLOAD",
    }

    // Write to STDOUT (Railway captures this)
    jsonBytes, _ := json.Marshal(entry)
    fmt.Println(string(jsonBytes)) 
}

func (s *Server) sendLeadToDiscord(email, name, role string) {
    webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
    if webhookURL == "" {
        // Log to stdout so you see it in Railway
        fmt.Println("⚠️ SKIPPING DISCORD: DISCORD_WEBHOOK_URL not set")
        return
    }

	logMsg := fmt.Sprintf(
		"🔔 **New Lead!**\n👤 **Name:** %s\n📧 **Email:** `%s`\n🛠 **Role:** %s\n🕒 **Time:** <t:%d:R>",
		name, email, role, time.Now().Unix(),
	)

	log.Println(logMsg)


	msg := DiscordMessage{
        Content: fmt.Sprintf(
            "🔔 **New Lead!**\n🛠 **Role:** %s\n🕒 **Time:** <t:%d:R>\n*Check Railway logs for contact info.*",
            role, time.Now().Unix(),
        ),
    }

    payload, _ := json.Marshal(msg)
    resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
    if err != nil {
        fmt.Printf("❌ Failed to send to Discord: %v\n", err)
        return
    }
    defer resp.Body.Close()
}

func (s *Server) handleCaptureLead(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	name := r.FormValue("from_name")
	role := r.FormValue("sender_role")

	// Send to Discord (Non-blocking)
	go s.sendLeadToDiscord(email, name, role)

	// Return the Javascript to trigger the print dialog
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<script>window.print();</script>`)
}
