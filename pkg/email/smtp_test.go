package email

import (
	"os"
	"testing"
)

func TestNewSMTPEmailService(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid configuration",
			env: map[string]string{
				"SMTP_HOST":     "smtp.mailtrap.io",
				"SMTP_PORT":     "587",
				"SMTP_USER":     "testuser",
				"SMTP_PASSWORD": "testpassword",
			},
			wantErr: false,
		},
		{
			name: "missing username",
			env: map[string]string{
				"SMTP_HOST":     "smtp.mailtrap.io",
				"SMTP_PORT":     "587",
				"SMTP_PASSWORD": "testpassword",
			},
			wantErr:     true,
			errContains: "SMTP credentials not configured",
		},
		{
			name: "missing password",
			env: map[string]string{
				"SMTP_HOST": "smtp.mailtrap.io",
				"SMTP_PORT": "587",
				"SMTP_USER": "testuser",
			},
			wantErr:     true,
			errContains: "SMTP credentials not configured",
		},
		{
			name: "missing both credentials",
			env: map[string]string{
				"SMTP_HOST": "smtp.mailtrap.io",
				"SMTP_PORT": "587",
			},
			wantErr:     true,
			errContains: "SMTP credentials not configured",
		},
		{
			name: "empty username",
			env: map[string]string{
				"SMTP_HOST":     "smtp.mailtrap.io",
				"SMTP_PORT":     "587",
				"SMTP_USER":     "",
				"SMTP_PASSWORD": "testpassword",
			},
			wantErr:     true,
			errContains: "SMTP credentials not configured",
		},
		{
			name: "empty password",
			env: map[string]string{
				"SMTP_HOST":     "smtp.mailtrap.io",
				"SMTP_PORT":     "587",
				"SMTP_USER":     "testuser",
				"SMTP_PASSWORD": "",
			},
			wantErr:     true,
			errContains: "SMTP credentials not configured",
		},
		{
			name: "missing host - uses default",
			env: map[string]string{
				"SMTP_PORT":     "587",
				"SMTP_USER":     "testuser",
				"SMTP_PASSWORD": "testpassword",
			},
			wantErr: false, // Should use default host smtp.mailtrap.io
		},
		{
			name: "invalid port - non-numeric",
			env: map[string]string{
				"SMTP_HOST":     "smtp.mailtrap.io",
				"SMTP_PORT":     "invalid",
				"SMTP_USER":     "testuser",
				"SMTP_PASSWORD": "testpassword",
			},
			wantErr: false, // Should fallback to default port 587
		},
		{
			name: "custom port",
			env: map[string]string{
				"SMTP_HOST":     "smtp.mailtrap.io",
				"SMTP_PORT":     "2525",
				"SMTP_USER":     "testuser",
				"SMTP_PASSWORD": "testpassword",
			},
			wantErr: false,
		},
		{
			name: "all custom values",
			env: map[string]string{
				"SMTP_HOST":              "smtp.custom.com",
				"SMTP_PORT":              "465",
				"SMTP_USER":              "customuser",
				"SMTP_PASSWORD":          "custompass",
				"SMTP_FROM_EMAIL":        "custom@example.com",
				"SMTP_FROM_NAME":         "Custom Name",
				"FRONTEND_URL":           "https://app.example.com",
				"EMAIL_VERIFICATION_URL": "https://app.example.com/verify",
				"RESET_PASSWORD_URL":     "https://app.example.com/reset",
				"SUPPORT_EMAIL":          "help@example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env vars
			originalEnv := make(map[string]string)
			envVars := []string{
				"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD",
				"SMTP_FROM_EMAIL", "SMTP_FROM_NAME", "FRONTEND_URL",
				"EMAIL_VERIFICATION_URL", "RESET_PASSWORD_URL", "SUPPORT_EMAIL",
			}
			for _, key := range envVars {
				originalEnv[key] = os.Getenv(key)
				os.Unsetenv(key) // Clear env var
			}

			// Set test env vars
			for key, value := range tt.env {
				os.Setenv(key, value)
			}

			// Test
			service, err := NewSMTPEmailService()

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSMTPEmailService() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("NewSMTPEmailService() error = %v, want error containing %q", err, tt.errContains)
				}
			}

			// If no error expected, check service is created
			if !tt.wantErr && service == nil {
				t.Error("NewSMTPEmailService() returned nil service without error")
			}

			// If service created successfully, validate config
			if !tt.wantErr && service != nil {
				if service.config == nil {
					t.Error("SMTPEmailService config is nil")
				}

				// Validate specific configs
				if username, ok := tt.env["SMTP_USER"]; ok && username != "" {
					if service.config.Username != username {
						t.Errorf("config.Username = %v, want %v", service.config.Username, username)
					}
				}

				if port, ok := tt.env["SMTP_PORT"]; ok && port == "2525" {
					if service.config.Port != 2525 {
						t.Errorf("config.Port = %v, want 2525", service.config.Port)
					}
				}

				if host, ok := tt.env["SMTP_HOST"]; ok && host != "" {
					if service.config.Host != host {
						t.Errorf("config.Host = %v, want %v", service.config.Host, host)
					}
				}
			}

			// Restore original env vars
			for key, value := range originalEnv {
				if value != "" {
					os.Setenv(key, value)
				} else {
					os.Unsetenv(key)
				}
			}
		})
	}
}

func TestSMTPConfigDefaults(t *testing.T) {
	// Clear all SMTP env vars
	envVars := []string{
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD",
		"SMTP_FROM_EMAIL", "SMTP_FROM_NAME", "FRONTEND_URL",
		"EMAIL_VERIFICATION_URL", "RESET_PASSWORD_URL", "SUPPORT_EMAIL",
	}
	originalEnv := make(map[string]string)
	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	// Set only required credentials for successful init
	os.Setenv("SMTP_USER", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpassword")

	defer func() {
		// Restore env
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			}
		}
	}()

	service, err := NewSMTPEmailService()
	if err != nil {
		t.Fatalf("NewSMTPEmailService() failed with valid credentials: %v", err)
	}

	// Check defaults
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"default host", service.config.Host, "smtp.mailtrap.io"},
		{"default port", string(rune(service.config.Port)), string(rune(587))},
		{"default from name", service.config.FromName, "Lakukan"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "default port" {
				if service.config.Port != 587 {
					t.Errorf("got port %d, want 587", service.config.Port)
				}
			} else if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			want:         "custom",
		},
		{
			name:         "env var not set - use default",
			key:          "TEST_VAR_MISSING",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
		{
			name:         "empty default",
			key:          "TEST_EMPTY",
			defaultValue: "",
			envValue:     "",
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and clear env var
			original := os.Getenv(tt.key)
			os.Unsetenv(tt.key)

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			got := getEnv(tt.key, tt.defaultValue)

			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}

			// Restore
			if original != "" {
				os.Setenv(tt.key, original)
			} else {
				os.Unsetenv(tt.key)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
