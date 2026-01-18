package worker

import (
	"log"
	"time"

	"sendmynotice/internal/email"
	"sendmynotice/internal/storage"
)

type EmailRunner struct {
	db          *storage.DB
	emailClient *email.Client
	campaign    []email.CampaignStep
}

func NewEmailRunner(db *storage.DB, emailClient *email.Client) *EmailRunner {
	return &EmailRunner{
		db:          db,
		emailClient: emailClient,
		campaign:    email.GetCampaign(),
	}
}

func (r *EmailRunner) Start() {
	log.Println("📧 Email Drip Worker Started...")
	
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		r.processCampaign()
	}
}

func (r *EmailRunner) processCampaign() {
	for _, step := range r.campaign {
		targetCurrentStep := step.StepID - 1
		
		leads, err := r.db.GetStaleLeads(step.Delay, targetCurrentStep)
		if err != nil {
			log.Printf("Error fetching leads for step %d: %v", step.StepID, err)
			continue
		}

		if len(leads) > 0 {
			log.Printf("🔍 Found %d leads ready for Email #%d", len(leads), step.StepID)
		}

		for _, lead := range leads {
			err := r.emailClient.Send(lead.Email, step.Subject, step.Body)
			if err != nil {
				log.Printf("Failed to send email to %s: %v", lead.Email, err)
				continue
			}

			if err := r.db.IncrementEmailStep(lead.ID, step.StepID); err != nil {
				log.Printf("Failed to update step for %s: %v", lead.Email, err)
			} else {
				log.Printf("✅ Sent Email #%d to %s", step.StepID, lead.Email)
			}
		}
	}
}