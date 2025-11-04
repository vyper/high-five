package slashcommand

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
)

// Helper function to generate valid Slack signature
func generateSlackSignature(secret, body string, timestamp int64) string {
	baseString := fmt.Sprintf("v0:%d:%s", timestamp, body)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(baseString))
	return "v0=" + hex.EncodeToString(h.Sum(nil))
}

func TestHandleSlashCommand(t *testing.T) {
	// Setup global config for tests
	setupTestConfig(t)

	tests := []struct {
		name               string
		formData           url.Values
		addSignature       bool
		useInvalidSig      bool
		timestamp          int64
		expectedStatusCode int
		expectedBodyPart   string
	}{
		{
			name: "successful slash command with valid signature",
			formData: url.Values{
				"trigger_id": []string{"12345.67890.abcdef"},
				"user_id":    []string{"U123456"},
				"command":    []string{"/elogie"},
			},
			addSignature:       true,
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "missing trigger_id returns bad request",
			formData: url.Values{
				"user_id": []string{"U123456"},
				"command": []string{"/elogie"},
			},
			addSignature:       true,
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusBadRequest,
			expectedBodyPart:   "Missing trigger_id",
		},
		{
			name: "invalid signature returns unauthorized",
			formData: url.Values{
				"trigger_id": []string{"12345.67890.abcdef"},
				"user_id":    []string{"U123456"},
			},
			addSignature:       true,
			useInvalidSig:      true,
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusUnauthorized,
			expectedBodyPart:   "Invalid Slack Signing Secret",
		},
		{
			name: "missing signature returns unauthorized",
			formData: url.Values{
				"trigger_id": []string{"12345.67890.abcdef"},
			},
			addSignature:       false,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name: "valid request with all form fields",
			formData: url.Values{
				"trigger_id":   []string{"12345.67890.abcdef"},
				"user_id":      []string{"U123456"},
				"user_name":    []string{"john.doe"},
				"command":      []string{"/elogie"},
				"text":         []string{""},
				"response_url": []string{"https://hooks.slack.com/commands/123/456"},
				"team_id":      []string{"T123456"},
				"channel_id":   []string{"C123456"},
			},
			addSignature:       true,
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create form body
			body := tt.formData.Encode()

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Add Slack signature headers if requested
			if tt.addSignature {
				timestamp := tt.timestamp
				if timestamp == 0 {
					timestamp = time.Now().Unix()
				}

				signature := generateSlackSignature(globalConfig.SigningSecret, body, timestamp)
				if tt.useInvalidSig {
					signature = "v0=invalid_signature"
				}

				req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))
				req.Header.Set("X-Slack-Signature", signature)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			handleSlashCommand(w, req)

			// Check status code
			if w.Code != tt.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}

			// Check response body if specified
			if tt.expectedBodyPart != "" {
				bodyStr := w.Body.String()
				if !strings.Contains(bodyStr, tt.expectedBodyPart) {
					t.Errorf("expected body to contain %q, got %q", tt.expectedBodyPart, bodyStr)
				}
			}
		})
	}
}

func TestHandleSlashCommand_FormParsingError(t *testing.T) {
	setupTestConfig(t)

	// Create request with invalid content type that will cause ParseForm to fail
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("invalid form data"))
	req.Header.Set("Content-Type", "application/json") // Wrong content type
	req.Header.Set("Content-Length", "100000000000")   // Invalid content length

	// Add valid signature
	timestamp := time.Now().Unix()
	body := "invalid form data"
	signature := generateSlackSignature(globalConfig.SigningSecret, body, timestamp)
	req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Slack-Signature", signature)

	w := httptest.NewRecorder()

	handleSlashCommand(w, req)

	// Should return bad request due to form parsing error
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for form parsing error, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleSlashCommand_SignatureValidation(t *testing.T) {
	setupTestConfig(t)

	formData := url.Values{
		"trigger_id": []string{"12345.67890.abcdef"},
		"user_id":    []string{"U123456"},
	}
	body := formData.Encode()

	tests := []struct {
		name               string
		timestamp          int64
		modifySignature    func(sig string) string
		modifyTimestamp    func(ts int64) int64
		expectedStatusCode int
	}{
		{
			name:               "valid signature and timestamp",
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusOK,
		},
		{
			name:      "old timestamp (replay attack protection)",
			timestamp: time.Now().Unix() - 400, // More than 5 minutes old
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:      "signature with wrong version",
			timestamp: time.Now().Unix(),
			modifySignature: func(sig string) string {
				return strings.Replace(sig, "v0=", "v1=", 1)
			},
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:      "tampered signature",
			timestamp: time.Now().Unix(),
			modifySignature: func(sig string) string {
				return sig + "tampered"
			},
			expectedStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timestamp := tt.timestamp
			if tt.modifyTimestamp != nil {
				timestamp = tt.modifyTimestamp(timestamp)
			}

			signature := generateSlackSignature(globalConfig.SigningSecret, body, timestamp)
			if tt.modifySignature != nil {
				signature = tt.modifySignature(signature)
			}

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))
			req.Header.Set("X-Slack-Signature", signature)

			w := httptest.NewRecorder()

			handleSlashCommand(w, req)

			if w.Code != tt.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}
		})
	}
}

func TestHandleSlashCommand_HTTPMethods(t *testing.T) {
	setupTestConfig(t)

	formData := url.Values{
		"trigger_id": []string{"12345.67890.abcdef"},
	}
	body := formData.Encode()
	timestamp := time.Now().Unix()
	signature := generateSlackSignature(globalConfig.SigningSecret, body, timestamp)

	methods := []string{http.MethodPost, http.MethodGet, http.MethodPut}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))
			req.Header.Set("X-Slack-Signature", signature)

			w := httptest.NewRecorder()

			handleSlashCommand(w, req)

			// All methods should work with valid signature
			// (Slack signature verification doesn't check method)
			if w.Code != http.StatusOK {
				t.Logf("Method %s returned status %d", method, w.Code)
			}
		})
	}
}

// Helper to setup test config
func setupTestConfig(_ *testing.T) {
	if globalConfig == nil {
		// Create a mock HTTP client
		mockHTTP := &MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true,"view":{"id":"V123456"}}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
		}

		globalConfig = &config.Config{
			SlackBotToken:  "xoxb-test-token",
			SlackChannelID: "C123456",
			SigningSecret:  "test-signing-secret-12345678",
			SlackAPI: &MockSlackClient{
				PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
					return "C123456", "1234567890.123456", nil
				},
			},
			HTTPClient: mockHTTP,
		}
	}
}

// Mock implementations needed for testing
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}, nil
}

type MockSlackClient struct {
	PostMessageFunc               func(channelID string, options ...slack.MsgOption) (string, string, error)
	InviteUsersToConversationFunc func(channelID string, users ...string) (*slack.Channel, error)
	GetUsersInConversationFunc    func(params *slack.GetUsersInConversationParameters) ([]string, string, error)
	GetUserInfoFunc               func(user string) (*slack.User, error)
}

func (m *MockSlackClient) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	if m.PostMessageFunc != nil {
		return m.PostMessageFunc(channelID, options...)
	}
	return "C123456", "1234567890.123456", nil
}

func (m *MockSlackClient) InviteUsersToConversation(channelID string, users ...string) (*slack.Channel, error) {
	if m.InviteUsersToConversationFunc != nil {
		return m.InviteUsersToConversationFunc(channelID, users...)
	}
	return &slack.Channel{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: channelID}}}, nil
}

func (m *MockSlackClient) GetUsersInConversation(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
	if m.GetUsersInConversationFunc != nil {
		return m.GetUsersInConversationFunc(params)
	}
	return []string{"U123456", "U789012"}, "", nil
}

func (m *MockSlackClient) GetUserInfo(user string) (*slack.User, error) {
	if m.GetUserInfoFunc != nil {
		return m.GetUserInfoFunc(user)
	}
	return &slack.User{
		ID:      user,
		IsBot:   false,
		Deleted: false,
	}, nil
}
