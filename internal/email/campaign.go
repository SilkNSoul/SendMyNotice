package email

import (
	"fmt"
	"time"
)

type CampaignStep struct {
	StepID  int
	Delay   time.Duration 
	Subject string
	Body    string 
}

func GetCampaign() []CampaignStep {
	link := "https://sendmynotice.com"
	footer := fmt.Sprintf(`<br><br><p style="font-size:10px; color:#999;">To stop these reminders, simply ignore this email. We stop automatically after day 20.</p>`)

	// Helper to make the list cleaner
	mkStep := func(id int, hours int, subj, body string) CampaignStep {
		return CampaignStep{
			StepID:  id,
			Delay:   time.Duration(hours) * time.Hour,
			Subject: subj,
			Body:    body + footer,
		}
	}

	return []CampaignStep{
		// EMAIL 1: 1 Hour after abandonment
		mkStep(1, 1, "Did you forget to file your Notice?", 
			`<p>You started a California Preliminary Notice but didn't finish.</p><p><strong>Remember: The 20-day clock is ticking.</strong> If you don't send this notice within 20 days of starting work, you legally forfeit your lien rights.</p><p><a href="`+link+`">Click here to finish and send it via Certified Mail</a>.</p>`),

		// EMAIL 2: Day 1 (24 hours after prev)
		mkStep(2, 24, "Don't risk your invoice", 
			`<p>80% of unpaid contractors lose their case because they missed the paperwork deadline.</p><p>A Preliminary Notice is the <strong>only way</strong> to secure your right to a Mechanic's Lien.</p><p>For $29, is it worth the risk?</p><p><a href="`+link+`">Protect your payments now</a>.</p>`),

		// EMAIL 3: Day 2
		mkStep(3, 24, "Why lawyers charge $350 for this", 
			`<p>We are not lawyers, but we know their pricing. A typical construction attorney charges $350/hour to draft the exact same document we generate for $29.</p><p>Save your money. Save your time.</p><p><a href="`+link+`">Send your notice in 60 seconds</a>.</p>`),

		// EMAIL 4: Day 3
		mkStep(4, 24, "It's not personal, it's business", 
			`<p>Contractors worry that sending a notice will make the homeowner mad.</p><p><strong>The Truth:</strong> Professional contractors send these on <em>every single job</em>. It shows you know the law and you expect to be paid.</p><p><a href="`+link+`">Send the notice</a>.</p>`),

		// EMAIL 5: Day 4
		mkStep(5, 24, "Day 4: Do you have the tracking number?", 
			`<p>If you mailed the notice yourself, do you have the Certified Mail tracking number filed away?</p><p>If not, you have no proof. We automatically digitalize and email you the tracking number instantly.</p><p><a href="`+link+`">Let us handle the paperwork</a>.</p>`),

		// EMAIL 6: Day 5
		mkStep(6, 24, "What happens if they don't pay?", 
			`<p>If you don't send a Preliminary Notice, and they don't pay you, there is <strong>nothing</strong> you can do to lien the property.</p><p>This document is your insurance policy.</p><p><a href="`+link+`">Get Insured</a>.</p>`),

		// EMAIL 7: Day 6
		mkStep(7, 24, "One week down...", 
			`<p>You are roughly one week into your filing window. The 20-day deadline is strict. There are no extensions.</p><p><a href="`+link+`">File Today</a>.</p>`),

		// EMAIL 8: Day 7
		mkStep(8, 24, "The 'Nice Guy' Trap", 
			`<p>Many contractors try to be the 'Nice Guy' and skip the notice. These are the contractors who get stiffed first when the money runs out.</p><p>Be the Smart Guy.</p><p><a href="`+link+`">Send the Notice</a>.</p>`),

		// EMAIL 9: Day 8
		mkStep(9, 24, "Documentation beats Conversation", 
			`<p>You can talk to the owner all day. But in court, only written documentation matters. Get your documentation on the record.</p><p><a href="`+link+`">Create Paper Trail</a>.</p>`),

		// EMAIL 10: Day 9
		mkStep(10, 24, "Civil Code 8200 Reminder", 
			`<p>California Civil Code 8200 mandates this notice. It is not aggressive; it is compliance.</p><p><a href="`+link+`">Comply Now</a>.</p>`),

		// EMAIL 11: Day 10 (Halfway Warning)
		mkStep(11, 24, "⚠️ 10 Days Left (Halfway Mark)", 
			`<p>You have 10 days remaining to file a fully compliant Preliminary Notice for work started 10 days ago.</p><p>Your window is closing.</p><p><a href="`+link+`">Secure your lien rights</a>.</p>`),

		// EMAIL 12: Day 11
		mkStep(12, 24, "Don't let them win", 
			`<p>Bad clients rely on you being lazy with paperwork. Don't give them that satisfaction.</p><p><a href="`+link+`">File Now</a>.</p>`),

		// EMAIL 13: Day 12
		mkStep(13, 24, "Is $29 too much?", 
			`<p>Is $29 too much to protect $5,000? It's less than the cost of a tank of gas.</p><p><a href="`+link+`">Send it</a>.</p>`),

		// EMAIL 14: Day 13
		mkStep(14, 24, "2 Weeks have passed", 
			`<p>If you started work 14 days ago, you have less than a week to file.</p><p><a href="`+link+`">File Now</a>.</p>`),

		// EMAIL 15: Day 14
		mkStep(15, 24, "Urgency: 6 Days Remaining", 
			`<p>The post office takes time. We process instantly, but you are cutting it close.</p><p><a href="`+link+`">Send via Certified Mail</a>.</p>`),

		// EMAIL 16: Day 15
		mkStep(16, 24, "URGENT: 5 Days Left", 
			`<p>This is your 5-day warning. You are in the red zone.</p><p><a href="`+link+`">File Immediately</a>.</p>`),

		// EMAIL 17: Day 16
		mkStep(17, 24, "4 Days Left", 
			`<p>Tick tock.</p><p><a href="`+link+`">Send My Notice</a>.</p>`),

		// EMAIL 18: Day 17
		mkStep(18, 24, "3 Days Left", 
			`<p>Do not wait until the last day.</p><p><a href="`+link+`">File Now</a>.</p>`),

		// EMAIL 19: Day 18
		mkStep(19, 24, "48 Hours Remaining", 
			`<p>If you don't file soon, you will likely lose your lien rights for the first days of labor.</p><p><a href="`+link+`">Send Now</a>.</p>`),

		// EMAIL 20: Day 19 (Last Call)
		mkStep(20, 24, "FINAL NOTICE: 24 Hours Left", 
			`<p>This is it. If you started work 20 days ago, today is your deadline.</p><p>Stop what you are doing. Protect your money.</p><p><a href="`+link+`">SEND IT NOW</a>.</p>`),
	}
}