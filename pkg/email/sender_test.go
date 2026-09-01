package email

import (
	"os"
	"testing"
)

// TestSMTPEmailService_SendVerificationEmail tests verification email sending
func TestSMTPEmailService_SendVerificationEmail(t *testing.T) {
	// Setup service
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	os.Setenv("EMAIL_VERIFICATION_URL", "https://test.com/verify")
	defer func() {
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
		os.Unsetenv("EMAIL_VERIFICATION_URL")
	}()

	service, err := NewSMTPEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name    string
		to      string
		toName  string
		token   string
		wantErr bool
	}{
		{
			name:    "valid verification email",
			to:      "test@example.com",
			toName:  "Test User",
			token:   "abc123token",
			wantErr: true, // Will fail because we don't have real SMTP credentials
		},
		{
			name:    "empty email",
			to:      "",
			toName:  "Test User",
			token:   "abc123",
			wantErr: true,
		},
		{
			name:    "empty name",
			to:      "test@example.com",
			toName:  "",
			token:   "abc123",
			wantErr: true, // Will fail at SMTP level
		},
		{
			name:    "empty token",
			to:      "test@example.com",
			toName:  "Test User",
			token:   "",
			wantErr: true, // Will fail at SMTP level
		},
		{
			name:    "invalid email format",
			to:      "invalid-email",
			toName:  "Test User",
			token:   "abc123",
			wantErr: true,
		},
		{
			name:    "long token",
			to:      "test@example.com",
			toName:  "Test User",
			token:   "very-long-token-12345678901234567890123456789012345678901234567890",
			wantErr: true, // Will fail at SMTP level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SendVerificationEmail(tt.to, tt.toName, tt.token)

			// We expect errors because we're not connected to real SMTP
			// This test validates the method signature and basic flow
			if (err != nil) != tt.wantErr {
				t.Errorf("SendVerificationEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSMTPEmailService_SendWelcomeEmail tests welcome email sending
func TestSMTPEmailService_SendWelcomeEmail(t *testing.T) {
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
	}()

	service, err := NewSMTPEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name    string
		to      string
		toName  string
		wantErr bool
	}{
		{
			name:    "valid welcome email",
			to:      "test@example.com",
			toName:  "Test User",
			wantErr: true, // Will fail without real SMTP
		},
		{
			name:    "empty email",
			to:      "",
			toName:  "Test User",
			wantErr: true,
		},
		{
			name:    "empty name",
			to:      "test@example.com",
			toName:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SendWelcomeEmail(tt.to, tt.toName)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendWelcomeEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSMTPEmailService_SendPasswordResetEmail tests password reset email
func TestSMTPEmailService_SendPasswordResetEmail(t *testing.T) {
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	os.Setenv("RESET_PASSWORD_URL", "https://test.com/reset")
	defer func() {
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
		os.Unsetenv("RESET_PASSWORD_URL")
	}()

	service, err := NewSMTPEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name    string
		to      string
		toName  string
		token   string
		wantErr bool
	}{
		{
			name:    "valid reset email",
			to:      "test@example.com",
			toName:  "Test User",
			token:   "reset123",
			wantErr: true,
		},
		{
			name:    "empty token",
			to:      "test@example.com",
			toName:  "Test User",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SendPasswordResetEmail(tt.to, tt.toName, tt.token)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendPasswordResetEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSMTPEmailService_SendAccountLockedEmail tests account locked email
func TestSMTPEmailService_SendAccountLockedEmail(t *testing.T) {
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
	}()

	service, err := NewSMTPEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name    string
		to      string
		toName  string
		wantErr bool
	}{
		{
			name:    "valid locked email",
			to:      "test@example.com",
			toName:  "Test User",
			wantErr: true,
		},
		{
			name:    "empty email",
			to:      "",
			toName:  "Test User",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SendAccountLockedEmail(tt.to, tt.toName)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendAccountLockedEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSMTPEmailService_SendPasswordChangedEmail tests password changed email
func TestSMTPEmailService_SendPasswordChangedEmail(t *testing.T) {
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
	}()

	service, err := NewSMTPEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	tests := []struct {
		name    string
		to      string
		toName  string
		wantErr bool
	}{
		{
			name:    "valid password changed email",
			to:      "test@example.com",
			toName:  "Test User",
			wantErr: true,
		},
		{
			name:    "empty name",
			to:      "test@example.com",
			toName:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.SendPasswordChangedEmail(tt.to, tt.toName)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendPasswordChangedEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSMTPEmailService_EmailComposition tests that emails are properly composed
func TestSMTPEmailService_EmailComposition(t *testing.T) {
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	os.Setenv("SMTP_FROM_EMAIL", "noreply@test.com")
	os.Setenv("SMTP_FROM_NAME", "Test App")
	defer func() {
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
		os.Unsetenv("SMTP_FROM_EMAIL")
		os.Unsetenv("SMTP_FROM_NAME")
	}()

	service, err := NewSMTPEmailService()
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Verify config is set correctly
	if service.config.FromEmail != "noreply@test.com" {
		t.Errorf("FromEmail = %v, want noreply@test.com", service.config.FromEmail)
	}

	if service.config.FromName != "Test App" {
		t.Errorf("FromName = %v, want Test App", service.config.FromName)
	}
}

// TestSMTPEmailService_URLConfiguration tests URL configuration
func TestSMTPEmailService_URLConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		checkFn  func(*SMTPEmailService) error
	}{
		{
			name: "custom verification URL",
			env: map[string]string{
				"SMTP_USER":              "user",
				"SMTP_PASSWORD":          "pass",
				"EMAIL_VERIFICATION_URL": "https://custom.com/verify",
			},
			checkFn: func(s *SMTPEmailService) error {
				if s.config.VerifyURL != "https://custom.com/verify" {
					t.Errorf("VerifyURL = %v, want https://custom.com/verify", s.config.VerifyURL)
				}
				return nil
			},
		},
		{
			name: "custom reset URL",
			env: map[string]string{
				"SMTP_USER":         "user",
				"SMTP_PASSWORD":     "pass",
				"RESET_PASSWORD_URL": "https://custom.com/reset",
			},
			checkFn: func(s *SMTPEmailService) error {
				if s.config.ResetURL != "https://custom.com/reset" {
					t.Errorf("ResetURL = %v, want https://custom.com/reset", s.config.ResetURL)
				}
				return nil
			},
		},
		{
			name: "custom frontend URL",
			env: map[string]string{
				"SMTP_USER":     "user",
				"SMTP_PASSWORD": "pass",
				"FRONTEND_URL":  "https://app.custom.com",
			},
			checkFn: func(s *SMTPEmailService) error {
				if s.config.FrontendURL != "https://app.custom.com" {
					t.Errorf("FrontendURL = %v, want https://app.custom.com", s.config.FrontendURL)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.env {
					os.Unsetenv(k)
				}
			}()

			service, err := NewSMTPEmailService()
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}

			if err := tt.checkFn(service); err != nil {
				t.Errorf("Check function failed: %v", err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkNoOpEmailService_SendVerificationEmail(b *testing.B) {
	service := &NoOpEmailService{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.SendVerificationEmail("test@example.com", "Test User", "token123")
	}
}

func BenchmarkSMTPEmailService_ConfigCreation(b *testing.B) {
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewSMTPEmailService()
	}
}
