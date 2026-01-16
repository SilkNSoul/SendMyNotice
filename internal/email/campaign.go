package email

import "time"

// CampaignStep defines a single email in the sequence
type CampaignStep struct {
	StepID  int
	Delay   time.Duration // How long to wait AFTER the previous step
	Subject string
	Body    string // Can contain HTML
}

// GetCampaign returns the sequence. 
// Step 0 is "Lead Created". 
// Step 1 is "1 Hour Reminder".
// Step 2 is "24 Hour Reminder", etc.
func GetCampaign() []CampaignStep {
	return []CampaignStep{
		{
			StepID:  1,
			Delay:   1 * time.Hour, // Sent 1 hour after they land on the site
			Subject: "Did you forget to send your Notice?",
			Body:    `<p>You started a Preliminary Notice but didn't finish.</p><p>Remember: The 20-day clock is ticking. <a href="https://sendmynotice.com">Finish it here</a>.</p>`,
		},
		{
			StepID:  2,
			Delay:   23 * time.Hour, // Sent 24 hours after Step 1 (Total 25 hours)
			Subject: "IMPORTANT: 20-Day Deadline Warning",
			Body:    `<p>Don't risk your lien rights over a $29 fee.</p><p>80% of contractors lose their money because they miss the deadline. <a href="https://sendmynotice.com">Send it now</a>.</p>`,
		},
		{
			StepID:  3,
			Delay:   48 * time.Hour, // Sent 2 days after Step 2
			Subject: "Why lawyers charge $350 for this",
			Body:    `<p>A lawyer charges $350/hour to do what our tool does in 30 seconds. Save your money. <a href="https://sendmynotice.com">Protect your invoice</a>.</p>`,
		},
        // ... ADD YOUR 20 EMAILS HERE ...
	}
}