package config

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		getenvFunc  func(string) string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, cfg *Config)
	}{
		{
			name: "successful config load with all variables",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":      "xoxb-test-token-123",
					"SLACK_CHANNEL_ID":     "C123456789",
					"SLACK_SIGNING_SECRET": "signing-secret-abc",
				}
				return env[key]
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg == nil {
					t.Fatal("config should not be nil")
				}

				if cfg.SlackBotToken != "xoxb-test-token-123" {
					t.Errorf("SlackBotToken = %q, want %q", cfg.SlackBotToken, "xoxb-test-token-123")
				}

				if cfg.SlackChannelID != "C123456789" {
					t.Errorf("SlackChannelID = %q, want %q", cfg.SlackChannelID, "C123456789")
				}

				if cfg.SigningSecret != "signing-secret-abc" {
					t.Errorf("SigningSecret = %q, want %q", cfg.SigningSecret, "signing-secret-abc")
				}

				if cfg.SlackAPI == nil {
					t.Error("SlackAPI should not be nil")
				}

				if cfg.HTTPClient == nil {
					t.Error("HTTPClient should not be nil")
				}
			},
		},
		{
			name: "missing SLACK_BOT_TOKEN",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_CHANNEL_ID":     "C123456789",
					"SLACK_SIGNING_SECRET": "signing-secret-abc",
				}
				return env[key]
			},
			wantErr:     true,
			errContains: "SLACK_BOT_TOKEN",
		},
		{
			name: "missing SLACK_CHANNEL_ID",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":      "xoxb-test-token-123",
					"SLACK_SIGNING_SECRET": "signing-secret-abc",
				}
				return env[key]
			},
			wantErr:     true,
			errContains: "SLACK_CHANNEL_ID",
		},
		{
			name: "missing SLACK_SIGNING_SECRET",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":  "xoxb-test-token-123",
					"SLACK_CHANNEL_ID": "C123456789",
				}
				return env[key]
			},
			wantErr:     true,
			errContains: "SLACK_SIGNING_SECRET",
		},
		{
			name: "empty SLACK_BOT_TOKEN",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":      "",
					"SLACK_CHANNEL_ID":     "C123456789",
					"SLACK_SIGNING_SECRET": "signing-secret-abc",
				}
				return env[key]
			},
			wantErr:     true,
			errContains: "SLACK_BOT_TOKEN",
		},
		{
			name: "empty SLACK_CHANNEL_ID",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":      "xoxb-test-token-123",
					"SLACK_CHANNEL_ID":     "",
					"SLACK_SIGNING_SECRET": "signing-secret-abc",
				}
				return env[key]
			},
			wantErr:     true,
			errContains: "SLACK_CHANNEL_ID",
		},
		{
			name: "empty SLACK_SIGNING_SECRET",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":      "xoxb-test-token-123",
					"SLACK_CHANNEL_ID":     "C123456789",
					"SLACK_SIGNING_SECRET": "",
				}
				return env[key]
			},
			wantErr:     true,
			errContains: "SLACK_SIGNING_SECRET",
		},
		{
			name: "all variables empty",
			getenvFunc: func(key string) string {
				return ""
			},
			wantErr:     true,
			errContains: "SLACK_BOT_TOKEN",
		},
		{
			name: "variables with whitespace only are treated as empty",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":      "   ",
					"SLACK_CHANNEL_ID":     "C123456789",
					"SLACK_SIGNING_SECRET": "signing-secret-abc",
				}
				return env[key]
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.SlackBotToken != "   " {
					t.Errorf("SlackBotToken should preserve whitespace, got %q", cfg.SlackBotToken)
				}
			},
		},
		{
			name: "variables with special characters",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":      "xoxb-123-456-abc!@#$%",
					"SLACK_CHANNEL_ID":     "C123_ABC-456",
					"SLACK_SIGNING_SECRET": "secret!@#$%^&*()",
				}
				return env[key]
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if cfg.SlackBotToken != "xoxb-123-456-abc!@#$%" {
					t.Errorf("SlackBotToken = %q, want %q", cfg.SlackBotToken, "xoxb-123-456-abc!@#$%")
				}

				if cfg.SlackChannelID != "C123_ABC-456" {
					t.Errorf("SlackChannelID = %q, want %q", cfg.SlackChannelID, "C123_ABC-456")
				}

				if cfg.SigningSecret != "secret!@#$%^&*()" {
					t.Errorf("SigningSecret = %q, want %q", cfg.SigningSecret, "secret!@#$%^&*()")
				}
			},
		},
		{
			name: "very long variable values",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":      strings.Repeat("x", 1000),
					"SLACK_CHANNEL_ID":     strings.Repeat("C", 500),
					"SLACK_SIGNING_SECRET": strings.Repeat("s", 750),
				}
				return env[key]
			},
			wantErr: false,
			validate: func(t *testing.T, cfg *Config) {
				if len(cfg.SlackBotToken) != 1000 {
					t.Errorf("SlackBotToken length = %d, want 1000", len(cfg.SlackBotToken))
				}

				if len(cfg.SlackChannelID) != 500 {
					t.Errorf("SlackChannelID length = %d, want 500", len(cfg.SlackChannelID))
				}

				if len(cfg.SigningSecret) != 750 {
					t.Errorf("SigningSecret length = %d, want 750", len(cfg.SigningSecret))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(tt.getenvFunc)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadConfig() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("LoadConfig() error = %v, want error containing %q", err, tt.errContains)
				}

				if cfg != nil {
					t.Errorf("LoadConfig() returned config when error expected, got %+v", cfg)
				}
			} else {
				if err != nil {
					t.Errorf("LoadConfig() unexpected error = %v", err)
				}

				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestLoadConfig_SlackAPIInitialization(t *testing.T) {
	// Test that SlackAPI is properly initialized
	getenvFunc := func(key string) string {
		env := map[string]string{
			"SLACK_BOT_TOKEN":      "xoxb-test-token",
			"SLACK_CHANNEL_ID":     "C123456",
			"SLACK_SIGNING_SECRET": "secret",
		}
		return env[key]
	}

	cfg, err := LoadConfig(getenvFunc)
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	// Verify SlackAPI is not nil
	if cfg.SlackAPI == nil {
		t.Fatal("SlackAPI should not be nil")
	}

	// Verify it's a real slack client (not just the interface)
	_, ok := cfg.SlackAPI.(*slack.Client)
	if !ok {
		t.Errorf("SlackAPI should be *slack.Client type")
	}
}

func TestLoadConfig_HTTPClientInitialization(t *testing.T) {
	// Test that HTTPClient is properly initialized with timeout
	getenvFunc := func(key string) string {
		env := map[string]string{
			"SLACK_BOT_TOKEN":      "xoxb-test-token",
			"SLACK_CHANNEL_ID":     "C123456",
			"SLACK_SIGNING_SECRET": "secret",
		}
		return env[key]
	}

	cfg, err := LoadConfig(getenvFunc)
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error = %v", err)
	}

	// Verify HTTPClient is not nil
	if cfg.HTTPClient == nil {
		t.Fatal("HTTPClient should not be nil")
	}

	// Verify it's a real http.Client
	httpClient, ok := cfg.HTTPClient.(*http.Client)
	if !ok {
		t.Errorf("HTTPClient should be *http.Client type")
		return
	}

	// Verify timeout is set to 10 seconds
	expectedTimeout := time.Second * 10
	if httpClient.Timeout != expectedTimeout {
		t.Errorf("HTTPClient.Timeout = %v, want %v", httpClient.Timeout, expectedTimeout)
	}
}

func TestLoadConfig_ErrorMessages(t *testing.T) {
	tests := []struct {
		name            string
		getenvFunc      func(string) string
		expectedMessage string
	}{
		{
			name: "SLACK_BOT_TOKEN error message",
			getenvFunc: func(key string) string {
				return ""
			},
			expectedMessage: "SLACK_BOT_TOKEN environment variable is required",
		},
		{
			name: "SLACK_CHANNEL_ID error message",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN": "xoxb-test",
				}
				return env[key]
			},
			expectedMessage: "SLACK_CHANNEL_ID environment variable is required",
		},
		{
			name: "SLACK_SIGNING_SECRET error message",
			getenvFunc: func(key string) string {
				env := map[string]string{
					"SLACK_BOT_TOKEN":  "xoxb-test",
					"SLACK_CHANNEL_ID": "C123",
				}
				return env[key]
			},
			expectedMessage: "SLACK_SIGNING_SECRET environment variable is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(tt.getenvFunc)

			if err == nil {
				t.Fatal("LoadConfig() expected error, got nil")
			}

			if err.Error() != tt.expectedMessage {
				t.Errorf("LoadConfig() error = %q, want %q", err.Error(), tt.expectedMessage)
			}
		})
	}
}

func TestLoadConfig_ReturnsNewInstanceEachTime(t *testing.T) {
	// Verify that LoadConfig returns a new instance each time
	getenvFunc := func(key string) string {
		env := map[string]string{
			"SLACK_BOT_TOKEN":      "xoxb-test-token",
			"SLACK_CHANNEL_ID":     "C123456",
			"SLACK_SIGNING_SECRET": "secret",
		}
		return env[key]
	}

	cfg1, err1 := LoadConfig(getenvFunc)
	if err1 != nil {
		t.Fatalf("LoadConfig() first call error = %v", err1)
	}

	cfg2, err2 := LoadConfig(getenvFunc)
	if err2 != nil {
		t.Fatalf("LoadConfig() second call error = %v", err2)
	}

	// Verify they are different instances
	if cfg1 == cfg2 {
		t.Error("LoadConfig() should return different instances, got same pointer")
	}

	// Verify values are the same
	if cfg1.SlackBotToken != cfg2.SlackBotToken {
		t.Error("LoadConfig() instances should have same SlackBotToken")
	}

	if cfg1.SlackChannelID != cfg2.SlackChannelID {
		t.Error("LoadConfig() instances should have same SlackChannelID")
	}

	if cfg1.SigningSecret != cfg2.SigningSecret {
		t.Error("LoadConfig() instances should have same SigningSecret")
	}
}
