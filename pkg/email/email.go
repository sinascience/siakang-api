package email

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	SendVerificationEmail(to, name, token string) error
	SendOTPVerificationEmail(to, name, otpCode string) error
	SendVerificationEmailWithOTP(to, name, token, otpCode string) error
	SendPasswordResetEmail(to, name, token string) error
	SendPasswordResetOTP(to, name, otpCode string) error
	SendWelcomeEmail(to, name string) error
	SendAccountLockedEmail(to, name string) error
	SendPasswordChangedEmail(to, name string) error
}

// EmailData represents common data for email templates
type EmailData struct {
	Name              string
	AppName           string
	AppURL            string
	VerificationURL   string
	ResetPasswordURL  string
	OTPCode           string
	SupportEmail      string
	Year              int
}

// renderTemplate renders an email template with the given data
func renderTemplate(templateName string, data interface{}) (string, error) {
	// Get the template file path
	templatePath := filepath.Join("pkg", "email", "templates", templateName)

	// Parse the template
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

// NoOpEmailService is a no-op implementation of EmailService (for development)
type NoOpEmailService struct{}

func (s *NoOpEmailService) SendVerificationEmail(to, name, token string) error {
	fmt.Printf("[NoOpEmail] Verification email to %s (token: %s)\n", to, token)
	return nil
}

func (s *NoOpEmailService) SendOTPVerificationEmail(to, name, otpCode string) error {
	fmt.Printf("[NoOpEmail] OTP verification email to %s (OTP: %s)\n", to, otpCode)
	return nil
}

func (s *NoOpEmailService) SendVerificationEmailWithOTP(to, name, token, otpCode string) error {
	fmt.Printf("[NoOpEmail] Verification email with OTP to %s (token: %s, OTP: %s)\n", to, token, otpCode)
	return nil
}

func (s *NoOpEmailService) SendPasswordResetEmail(to, name, token string) error {
	fmt.Printf("[NoOpEmail] Password reset email to %s (token: %s)\n", to, token)
	return nil
}

func (s *NoOpEmailService) SendPasswordResetOTP(to, name, otpCode string) error {
	fmt.Printf("[NoOpEmail] Password reset OTP email to %s (OTP: %s)\n", to, otpCode)
	return nil
}

func (s *NoOpEmailService) SendWelcomeEmail(to, name string) error {
	fmt.Printf("[NoOpEmail] Welcome email to %s\n", to)
	return nil
}

func (s *NoOpEmailService) SendAccountLockedEmail(to, name string) error {
	fmt.Printf("[NoOpEmail] Account locked email to %s\n", to)
	return nil
}

func (s *NoOpEmailService) SendPasswordChangedEmail(to, name string) error {
	fmt.Printf("[NoOpEmail] Password changed email to %s\n", to)
	return nil
}
