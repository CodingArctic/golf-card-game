package service

import (
	"context"
	"fmt"
	"os"

	"github.com/resend/resend-go/v3"
)

// EmailService handles sending emails via Resend
type EmailService struct {
	client *resend.Client
}

// NewEmailService creates a new email service
func NewEmailService() *EmailService {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		// Return service with nil client - will skip email sending
		return &EmailService{client: nil}
	}

	return &EmailService{
		client: resend.NewClient(apiKey),
	}
}

// SendWelcomeEmail sends a welcome email to a newly registered user
func (s *EmailService) SendWelcomeEmail(toEmail, username string) error {
	if s.client == nil {
		return fmt.Errorf("RESEND_API_KEY not configured")
	}

	fromEmail := os.Getenv("RESEND_FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "onboarding@resend.dev" // Default Resend test email
	}

	ctx := context.Background()
	params := &resend.SendEmailRequest{
		From:    "Golf Card Game <" + fromEmail + ">",
		To:      []string{toEmail},
		Subject: "Welcome to Golf Card Game!",
		Html: fmt.Sprintf(`
			<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
				<h1 style="color: #2563eb;">Welcome to Golf Card Game, %s!</h1>
				<p>Thanks for creating an account. We're excited to have you join our community!</p>
				<p>You can now:</p>
				<ul>
					<li>Create and join games</li>
					<li>Invite friends to play</li>
					<li>Chat with other players in the lobby</li>
				</ul>
				<p>Ready to start playing? <a href="%s" style="color: #2563eb;">Log in now</a></p>
				<p>Need a developer? Check out <a href="https://clyde.biz" style="color: #2563eb;">my portfolio site</a>!</p>
				<hr style="margin: 30px 0; border: none; border-top: 1px solid #e5e7eb;">
				<p style="color: #6b7280; font-size: 12px;">
					This is an automated message. Please do not reply to this email.
				</p>
			</div>
		`, username, getAppURL()),
	}

	sent, err := s.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	fmt.Printf("Welcome email sent to %s (ID: %s)\n", toEmail, sent.Id)
	return nil
}

// getAppURL returns the application URL from environment or defaults to localhost
func getAppURL() string {
	url := os.Getenv("APP_URL")
	if url == "" {
		return "http://localhost:3000/login"
	}
	return url + "/login"
}
