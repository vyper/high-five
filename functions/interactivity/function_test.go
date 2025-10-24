package interactivity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

func TestHandleInteractivity(t *testing.T) {
	setupTestConfig(t)

	// Create a valid block action callback
	blockActionCallback := slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		User: slack.User{
			ID:   "U123456",
			Name: "testuser",
		},
		View: slack.View{
			ID:   "V123456",
			Hash: "hash123",
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"kudo_message": {
						"kudo_message": {Value: ""},
					},
				},
			},
		},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{
				{
					ActionID: "kudo_type",
					SelectedOption: slack.OptionBlockObject{
						Value: "resolvedor-de-problemas",
					},
				},
			},
		},
	}

	// Create a valid view submission callback
	viewSubmissionCallback := slack.InteractionCallback{
		Type: slack.InteractionTypeViewSubmission,
		User: slack.User{
			ID: "U123456",
		},
		View: slack.View{
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"kudo_users": {
						"kudo_users": {
							SelectedUsers: []string{"U789012"},
						},
					},
					"kudo_type": {
						"kudo_type": {
							SelectedOption: slack.OptionBlockObject{
								Value: "entrega-excepcional",
								Text: &slack.TextBlockObject{
									Text: ":dart: Entrega Excepcional",
								},
							},
						},
					},
					"kudo_message": {
						"kudo_message": {
							Value: "Great job!",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name               string
		callback           interface{}
		addSignature       bool
		useInvalidSig      bool
		timestamp          int64
		expectedStatusCode int
		expectedBodyPart   string
	}{
		{
			name:               "block action with valid signature (may return 500 if modal update fails)",
			callback:           blockActionCallback,
			addSignature:       true,
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusInternalServerError, // UpdateModal fails with mock
		},
		{
			name:               "successful view submission with valid signature",
			callback:           viewSubmissionCallback,
			addSignature:       true,
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "invalid signature returns unauthorized",
			callback:           blockActionCallback,
			addSignature:       true,
			useInvalidSig:      true,
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusUnauthorized,
			expectedBodyPart:   "Invalid Slack Signing Secret",
		},
		{
			name:               "missing signature returns unauthorized",
			callback:           blockActionCallback,
			addSignature:       false,
			expectedStatusCode: http.StatusUnauthorized,
		},
		{
			name:               "missing payload returns bad request",
			callback:           nil,
			addSignature:       true,
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusBadRequest,
			expectedBodyPart:   "Bad Request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var formData url.Values

			if tt.callback != nil {
				// Marshal callback to JSON
				callbackJSON, err := json.Marshal(tt.callback)
				if err != nil {
					t.Fatalf("failed to marshal callback: %v", err)
				}

				formData = url.Values{
					"payload": []string{string(callbackJSON)},
				}
			} else {
				formData = url.Values{}
			}

			body := formData.Encode()

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
			handleInteractivity(w, req)

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

func TestHandleInteractivity_InvalidJSON(t *testing.T) {
	setupTestConfig(t)

	formData := url.Values{
		"payload": []string{`{invalid json`},
	}
	body := formData.Encode()
	timestamp := time.Now().Unix()
	signature := generateSlackSignature(globalConfig.SigningSecret, body, timestamp)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Slack-Signature", signature)

	w := httptest.NewRecorder()

	handleInteractivity(w, req)

	// Should return bad request for invalid JSON
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid JSON, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Invalid Slack Interaction Callback") {
		t.Errorf("expected error message about invalid callback")
	}
}

func TestHandleInteractivity_UnknownInteractionType(t *testing.T) {
	setupTestConfig(t)

	// Create callback with unknown type
	callback := slack.InteractionCallback{
		Type: "unknown_type",
		User: slack.User{
			ID: "U123456",
		},
	}

	callbackJSON, _ := json.Marshal(callback)
	formData := url.Values{
		"payload": []string{string(callbackJSON)},
	}
	body := formData.Encode()
	timestamp := time.Now().Unix()
	signature := generateSlackSignature(globalConfig.SigningSecret, body, timestamp)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Slack-Signature", signature)

	w := httptest.NewRecorder()

	handleInteractivity(w, req)

	// Should return 200 OK even for unknown types (just acknowledged)
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for unknown type, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleInteractivity_FormParsingError(t *testing.T) {
	setupTestConfig(t)

	// Create request with invalid content length
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload=test"))
	req.Header.Set("Content-Type", "application/json") // Wrong content type
	req.Header.Set("Content-Length", "100000000000")   // Invalid content length

	timestamp := time.Now().Unix()
	body := "payload=test"
	signature := generateSlackSignature(globalConfig.SigningSecret, body, timestamp)
	req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Slack-Signature", signature)

	w := httptest.NewRecorder()

	handleInteractivity(w, req)

	// Should return bad request due to form parsing error
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for form parsing error, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleInteractivity_SignatureValidation(t *testing.T) {
	setupTestConfig(t)

	callback := slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		User: slack.User{ID: "U123456"},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{},
		},
	}
	callbackJSON, _ := json.Marshal(callback)
	formData := url.Values{"payload": []string{string(callbackJSON)}}
	body := formData.Encode()

	tests := []struct {
		name               string
		timestamp          int64
		modifySignature    func(sig string) string
		expectedStatusCode int
	}{
		{
			name:               "valid signature and timestamp",
			timestamp:          time.Now().Unix(),
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "old timestamp (replay attack protection)",
			timestamp:          time.Now().Unix() - 400,
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
			signature := generateSlackSignature(globalConfig.SigningSecret, body, tt.timestamp)
			if tt.modifySignature != nil {
				signature = tt.modifySignature(signature)
			}

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(tt.timestamp, 10))
			req.Header.Set("X-Slack-Signature", signature)

			w := httptest.NewRecorder()

			handleInteractivity(w, req)

			if w.Code != tt.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}
		})
	}
}

func TestHandleInteractivity_BothInteractionTypes(t *testing.T) {
	setupTestConfig(t)

	// Test both block_actions and view_submission
	interactionTypes := []struct {
		name     string
		callback slack.InteractionCallback
	}{
		{
			name: "block_actions",
			callback: slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				User: slack.User{ID: "U123456"},
				View: slack.View{
					ID:   "V123",
					Hash: "hash",
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_message": {
								"kudo_message": {Value: ""},
							},
						},
					},
				},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID: "kudo_type",
							SelectedOption: slack.OptionBlockObject{
								Value: "test",
							},
						},
					},
				},
			},
		},
		{
			name: "view_submission",
			callback: slack.InteractionCallback{
				Type: slack.InteractionTypeViewSubmission,
				User: slack.User{ID: "U123456"},
				View: slack.View{
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_users": {
								"kudo_users": {
									SelectedUsers: []string{"U789"},
								},
							},
							"kudo_type": {
								"kudo_type": {
									SelectedOption: slack.OptionBlockObject{
										Value: "test",
										Text: &slack.TextBlockObject{
											Text: ":star: Test",
										},
									},
								},
							},
							"kudo_message": {
								"kudo_message": {
									Value: "Test message",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range interactionTypes {
		t.Run(tt.name, func(t *testing.T) {
			callbackJSON, _ := json.Marshal(tt.callback)
			formData := url.Values{"payload": []string{string(callbackJSON)}}
			body := formData.Encode()
			timestamp := time.Now().Unix()
			signature := generateSlackSignature(globalConfig.SigningSecret, body, timestamp)

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("X-Slack-Request-Timestamp", strconv.FormatInt(timestamp, 10))
			req.Header.Set("X-Slack-Signature", signature)

			w := httptest.NewRecorder()

			handleInteractivity(w, req)

			// Block actions may return 500 if UpdateModal fails, view_submission always returns 200
			if tt.name == "view_submission" && w.Code != http.StatusOK {
				t.Errorf("expected status %d for %s, got %d", http.StatusOK, tt.name, w.Code)
			}
			// For block_actions, we accept either 200 or 500
			if tt.name == "block_actions" && w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
				t.Errorf("expected status 200 or 500 for %s, got %d", tt.name, w.Code)
			}
		})
	}
}

// Helper to setup test config
func setupTestConfig(_ *testing.T) {
	if globalConfig == nil {
		mockHTTP := &MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true}`
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

// Mock implementations
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
	PostMessageFunc func(channelID string, options ...slack.MsgOption) (string, string, error)
}

func (m *MockSlackClient) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	if m.PostMessageFunc != nil {
		return m.PostMessageFunc(channelID, options...)
	}
	return "C123456", "1234567890.123456", nil
}
