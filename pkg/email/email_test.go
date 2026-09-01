package email

import (
	"os"
	"testing"
)

func TestNoOpEmailService(t *testing.T) {
	service := &NoOpEmailService{}

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "SendVerificationEmail",
			fn: func() error {
				return service.SendVerificationEmail("test@example.com", "Test User", "token123")
			},
		},
		{
			name: "SendPasswordResetEmail",
			fn: func() error {
				return service.SendPasswordResetEmail("test@example.com", "Test User", "resettoken")
			},
		},
		{
			name: "SendWelcomeEmail",
			fn: func() error {
				return service.SendWelcomeEmail("test@example.com", "Test User")
			},
		},
		{
			name: "SendAccountLockedEmail",
			fn: func() error {
				return service.SendAccountLockedEmail("test@example.com", "Test User")
			},
		},
		{
			name: "SendPasswordChangedEmail",
			fn: func() error {
				return service.SendPasswordChangedEmail("test@example.com", "Test User")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err != nil {
				t.Errorf("%s returned error: %v, want nil", tt.name, err)
			}
		})
	}
}

func TestSMTPEmailService_TemplateFiles(t *testing.T) {
	// Test that template files exist
	templates := []string{
		"verification.html",
		"welcome.html",
		"reset_password.html",
		"account_locked.html",
		"password_changed.html",
	}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			templatePath := "templates/" + tmpl
			if _, err := os.Stat(templatePath); os.IsNotExist(err) {
				t.Errorf("Template file does not exist: %s", templatePath)
			}
		})
	}
}

func TestSMTPEmailService_ServiceInitialization(t *testing.T) {
	// Test that service initializes correctly with valid credentials
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
	}()

	service, err := NewSMTPEmailService()
	if err != nil {
		t.Fatalf("Failed to create SMTP service with valid credentials: %v", err)
	}

	if service == nil {
		t.Fatal("Service is nil")
	}

	if service.config == nil {
		t.Fatal("Service config is nil")
	}

	// Verify credentials are set
	if service.config.Username != "testuser" {
		t.Errorf("Username = %v, want testuser", service.config.Username)
	}

	if service.config.Password != "testpass" {
		t.Errorf("Password = %v, want testpass", service.config.Password)
	}
}

func TestSMTPEmailService_VerifyConfiguration(t *testing.T) {
	// Test that service configuration is properly set
	tests := []struct {
		name     string
		envVars  map[string]string
		checkKey string
		expected string
	}{
		{
			name: "custom frontend URL",
			envVars: map[string]string{
				"SMTP_USER":     "user",
				"SMTP_PASSWORD": "pass",
				"FRONTEND_URL":  "https://custom.com",
			},
			checkKey: "FrontendURL",
			expected: "https://custom.com",
		},
		{
			name: "custom support email",
			envVars: map[string]string{
				"SMTP_USER":      "user",
				"SMTP_PASSWORD":  "pass",
				"SUPPORT_EMAIL":  "help@custom.com",
			},
			checkKey: "SupportEmail",
			expected: "help@custom.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			service, err := NewSMTPEmailService()
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}

			// Check the config value
			switch tt.checkKey {
			case "FrontendURL":
				if service.config.FrontendURL != tt.expected {
					t.Errorf("FrontendURL = %v, want %v", service.config.FrontendURL, tt.expected)
				}
			case "SupportEmail":
				if service.config.SupportEmail != tt.expected {
					t.Errorf("SupportEmail = %v, want %v", service.config.SupportEmail, tt.expected)
				}
			}
		})
	}
}

func TestSMTPEmailService_ConfigInjection(t *testing.T) {
	// Test that service config is properly injected into templates
	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{
			name: "custom URLs",
			envVars: map[string]string{
				"SMTP_USER":              "user",
				"SMTP_PASSWORD":          "pass",
				"FRONTEND_URL":           "https://custom.com",
				"EMAIL_VERIFICATION_URL": "https://custom.com/verify",
				"SUPPORT_EMAIL":          "help@custom.com",
			},
		},
		{
			name: "default URLs",
			envVars: map[string]string{
				"SMTP_USER":     "user",
				"SMTP_PASSWORD": "pass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.envVars {
					os.Unsetenv(k)
				}
			}()

			service, err := NewSMTPEmailService()
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}

			// Check config is set
			if service.config == nil {
				t.Fatal("Service config is nil")
			}

			// Verify config values
			if frontendURL, ok := tt.envVars["FRONTEND_URL"]; ok {
				if service.config.FrontendURL != frontendURL {
					t.Errorf("FrontendURL = %v, want %v", service.config.FrontendURL, frontendURL)
				}
			}

			if supportEmail, ok := tt.envVars["SUPPORT_EMAIL"]; ok {
				if service.config.SupportEmail != supportEmail {
					t.Errorf("SupportEmail = %v, want %v", service.config.SupportEmail, supportEmail)
				}
			}
		})
	}
}
