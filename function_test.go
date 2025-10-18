package function

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

// MockHTTPClient implements HTTPClient for testing
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

// createTestConfig creates a test configuration with mocks
func createTestConfig() *Config {
	return &Config{
		SlackBotToken:  "xoxb-test-token",
		SlackChannelID: "C12345678",
		SigningSecret:  "test-signing-secret",
		SlackAPI:       &MockSlackClient{},
		HTTPClient:     &MockHTTPClient{},
	}
}

func TestLoadConfig_Success(t *testing.T) {
	getenv := func(key string) string {
		vars := map[string]string{
			"SLACK_BOT_TOKEN":      "xoxb-test-token",
			"SLACK_CHANNEL_ID":     "C12345678",
			"SLACK_SIGNING_SECRET": "test-signing-secret",
		}
		return vars[key]
	}

	config, err := LoadConfig(getenv)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if config.SlackBotToken != "xoxb-test-token" {
		t.Errorf("Expected SlackBotToken xoxb-test-token, got %s", config.SlackBotToken)
	}

	if config.SlackChannelID != "C12345678" {
		t.Errorf("Expected SlackChannelID C12345678, got %s", config.SlackChannelID)
	}

	if config.SigningSecret != "test-signing-secret" {
		t.Errorf("Expected SigningSecret test-signing-secret, got %s", config.SigningSecret)
	}

	if config.SlackAPI == nil {
		t.Error("Expected SlackAPI to be initialized")
	}

	if config.HTTPClient == nil {
		t.Error("Expected HTTPClient to be initialized")
	}
}

func TestLoadConfig_MissingVariables(t *testing.T) {
	tests := []struct {
		name        string
		getenv      func(string) string
		expectedErr string
	}{
		{
			name: "missing bot token",
			getenv: func(key string) string {
				vars := map[string]string{
					"SLACK_CHANNEL_ID":     "C12345678",
					"SLACK_SIGNING_SECRET": "test-signing-secret",
				}
				return vars[key]
			},
			expectedErr: "SLACK_BOT_TOKEN environment variable is required",
		},
		{
			name: "missing channel ID",
			getenv: func(key string) string {
				vars := map[string]string{
					"SLACK_BOT_TOKEN":      "xoxb-test-token",
					"SLACK_SIGNING_SECRET": "test-signing-secret",
				}
				return vars[key]
			},
			expectedErr: "SLACK_CHANNEL_ID environment variable is required",
		},
		{
			name: "missing signing secret",
			getenv: func(key string) string {
				vars := map[string]string{
					"SLACK_BOT_TOKEN":  "xoxb-test-token",
					"SLACK_CHANNEL_ID": "C12345678",
				}
				return vars[key]
			},
			expectedErr: "SLACK_SIGNING_SECRET environment variable is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadConfig(tt.getenv)

			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			if config != nil {
				t.Error("Expected nil config on error")
			}

			if err.Error() != tt.expectedErr {
				t.Errorf("Expected error %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestGiveKudos_InvalidSignature(t *testing.T) {
	config := createTestConfig()

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "missing signature headers",
			setupRequest: func() *http.Request {
				body := "trigger_id=12345.67890"
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid Slack Signin Secret",
		},
		{
			name: "invalid signature value",
			setupRequest: func() *http.Request {
				body := "trigger_id=12345.67890"
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
				req.Header.Set("X-Slack-Signature", "v0=invalid_signature")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid Slack Signin Secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			rr := httptest.NewRecorder()

			handleKudos(rr, req, config)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			if !strings.Contains(rr.Body.String(), tt.expectedBody) {
				t.Errorf("handler returned unexpected body: got %v want substring %v",
					rr.Body.String(), tt.expectedBody)
			}
		})
	}
}

func TestGiveKudos_InitialCommand_OpensModal(t *testing.T) {
	httpCallMade := false
	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			httpCallMade = true

			// Verify request properties
			if req.URL.String() != "https://slack.com/api/views.open" {
				t.Errorf("Expected URL https://slack.com/api/views.open, got %s", req.URL.String())
			}

			if req.Method != http.MethodPost {
				t.Errorf("Expected POST method, got %s", req.Method)
			}

			authHeader := req.Header.Get("Authorization")
			if authHeader != "Bearer xoxb-test-token" {
				t.Errorf("Expected Authorization header with Bearer token, got %s", authHeader)
			}

			body, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(body), "trigger_id") {
				t.Error("Expected request body to contain trigger_id")
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		},
	}

	body := "trigger_id=12345.67890"
	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	if !httpCallMade {
		t.Error("Expected HTTP call to be made")
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v, body: %s",
			status, http.StatusOK, rr.Body.String())
	}
}

func TestGiveKudos_InitialCommand_HTTPErrors(t *testing.T) {
	tests := []struct {
		name     string
		mockFunc func(req *http.Request) (*http.Response, error)
	}{
		{
			name: "error creating request",
			mockFunc: func(req *http.Request) (*http.Response, error) {
				// This tests the error handling in Do()
				return nil, errors.New("network error")
			},
		},
		{
			name: "error reading response body",
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(errReader{}),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			config.HTTPClient = &MockHTTPClient{DoFunc: tt.mockFunc}

			body := "trigger_id=12345.67890"
			req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
			req.Body = io.NopCloser(strings.NewReader(body))
			req.URL = &url.URL{Path: "/"}
			req.RequestURI = "/"

			rr := httptest.NewRecorder()

			handleKudos(rr, req, config)

			// Function should handle errors gracefully and return 200
			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, http.StatusOK)
			}
		})
	}
}

func TestGiveKudos_ModalSubmission_PostsMessage(t *testing.T) {
	postMessageCalled := false
	var capturedChannelID string

	config := createTestConfig()
	config.SlackAPI = &MockSlackClient{
		PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			postMessageCalled = true
			capturedChannelID = channelID
			return channelID, "1234567890.123456", nil
		},
	}

	payload := ValidInteractionCallbackPayload()
	formData := url.Values{}
	formData.Set("payload", payload)
	body := formData.Encode()

	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	if !postMessageCalled {
		t.Error("Expected PostMessage to be called")
	}

	if capturedChannelID != "C12345678" {
		t.Errorf("Expected channel ID C12345678, got %s", capturedChannelID)
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestGiveKudos_ModalSubmission_PostMessageError(t *testing.T) {
	config := createTestConfig()
	config.SlackAPI = &MockSlackClient{
		PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			return "", "", errors.New("slack API error")
		},
	}

	payload := ValidInteractionCallbackPayload()
	formData := url.Values{}
	formData.Set("payload", payload)
	body := formData.Encode()

	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	// Should handle error gracefully
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestGiveKudos_ModalSubmission_InvalidJSON(t *testing.T) {
	config := createTestConfig()

	payload := InvalidJSONPayload()
	formData := url.Values{}
	formData.Set("payload", payload)
	body := formData.Encode()

	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code for invalid JSON: got %v want %v",
			status, http.StatusUnauthorized)
	}

	if !strings.Contains(rr.Body.String(), "Invalid Slack Interaction Callback") {
		t.Errorf("Expected error message about invalid callback, got: %s", rr.Body.String())
	}
}

func TestGiveKudos_DifferentHTTPMethods(t *testing.T) {
	config := createTestConfig()

	tests := []struct {
		name           string
		method         string
		contentType    string
		body           string
		expectedStatus int
	}{
		{
			name:           "GET request",
			method:         http.MethodGet,
			contentType:    "",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PUT request",
			method:         http.MethodPut,
			contentType:    "application/json",
			body:           `{"test": "data"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with JSON content type",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"test": "data"}`,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := CreateSlackRequest(tt.method, tt.contentType, tt.body, config.SigningSecret)
			req.Body = io.NopCloser(strings.NewReader(tt.body))
			req.URL = &url.URL{Path: "/"}
			req.RequestURI = "/"

			rr := httptest.NewRecorder()

			handleKudos(rr, req, config)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}
		})
	}
}

func TestGiveKudos_ParseFormError(t *testing.T) {
	config := createTestConfig()

	body := "invalid_form_data=%ZZ"
	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	// Should complete despite parse error
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestGiveKudos_EmptyPayload(t *testing.T) {
	httpCallMade := false
	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			httpCallMade = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		},
	}

	formData := url.Values{}
	formData.Set("payload", "")
	formData.Set("trigger_id", "12345.67890")
	body := formData.Encode()

	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	if !httpCallMade {
		t.Error("Expected HTTP call to be made when payload is empty but trigger_id is present")
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestGenerateSlackSignature(t *testing.T) {
	secret := "test-secret"
	body := "test-body"
	timestamp := int64(1234567890)

	signature := GenerateSlackSignature(secret, body, timestamp)

	if !strings.HasPrefix(signature, "v0=") {
		t.Errorf("Expected signature to start with 'v0=', got: %s", signature)
	}

	// Verify signature is consistent
	signature2 := GenerateSlackSignature(secret, body, timestamp)
	if signature != signature2 {
		t.Error("Expected consistent signature generation")
	}

	// Verify different body produces different signature
	signature3 := GenerateSlackSignature(secret, "different-body", timestamp)
	if signature == signature3 {
		t.Error("Expected different signature for different body")
	}
}

func TestValidInteractionCallbackPayload(t *testing.T) {
	payload := ValidInteractionCallbackPayload()

	var callback slack.InteractionCallback
	if err := json.Unmarshal([]byte(payload), &callback); err != nil {
		t.Fatalf("Failed to unmarshal valid payload: %v", err)
	}

	if callback.User.ID != "U12345678" {
		t.Errorf("Expected user ID U12345678, got %s", callback.User.ID)
	}

	if callback.Type != "view_submission" {
		t.Errorf("Expected type view_submission, got %s", callback.Type)
	}

	kudoUsers := callback.View.State.Values["kudo_users"]["kudo_users"].SelectedUsers
	if len(kudoUsers) != 2 {
		t.Errorf("Expected 2 selected users, got %d", len(kudoUsers))
	}
}

func TestValidInteractionCallbackPayloadNoMessage(t *testing.T) {
	payload := ValidInteractionCallbackPayloadNoMessage()

	var callback slack.InteractionCallback
	if err := json.Unmarshal([]byte(payload), &callback); err != nil {
		t.Fatalf("Failed to unmarshal valid payload: %v", err)
	}

	message := callback.View.State.Values["kudo_message"]["kudo_message"].Value
	if message != "" {
		t.Errorf("Expected empty message, got: %s", message)
	}
}

func TestInvalidJSONPayload(t *testing.T) {
	payload := InvalidJSONPayload()

	var callback slack.InteractionCallback
	if err := json.Unmarshal([]byte(payload), &callback); err == nil {
		t.Error("Expected error when unmarshaling invalid JSON, got nil")
	}
}

// errReader simulates an error when reading
type errReader struct{}

func (errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

// TestGiveKudos_Wrapper tests the public giveKudos function
func TestGiveKudos_Wrapper(t *testing.T) {
	// Test that giveKudos properly calls handleKudos with globalConfig
	body := "trigger_id=12345.67890"
	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, "secret")
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	// This will use the globalConfig which was set during init
	giveKudos(rr, req)

	// Should handle the request (even if it fails due to test env)
	// The important thing is that it doesn't panic
	if rr.Code != http.StatusOK && rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 200 or 401, got %v", rr.Code)
	}
}

// TestMockSlackClient_DefaultBehavior tests the default mock behavior
func TestMockSlackClient_DefaultBehavior(t *testing.T) {
	mock := &MockSlackClient{}

	channelID, timestamp, err := mock.PostMessage("C123", slack.MsgOptionText("test", false))

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if channelID != "C123" {
		t.Errorf("Expected channel ID C123, got %s", channelID)
	}

	if timestamp == "" {
		t.Error("Expected timestamp to be returned")
	}
}

// TestMockHTTPClient_DefaultBehavior tests the default mock behavior
func TestMockHTTPClient_DefaultBehavior(t *testing.T) {
	mock := &MockHTTPClient{}

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	resp, err := mock.Do(req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") {
		t.Error("Expected response body to contain 'ok'")
	}
}

// TestGiveKudos_NonPOST_BodyReadError tests error reading body in else block
func TestGiveKudos_NonPOST_BodyReadError(t *testing.T) {
	config := createTestConfig()

	req := CreateSlackRequest(http.MethodGet, "", "", config.SigningSecret)
	req.Body = io.NopCloser(errReader{})
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	// Should handle error gracefully and return 200
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

// TestGiveKudos_NonPOST_WithBody tests successful body read in else block
func TestGiveKudos_NonPOST_WithBody(t *testing.T) {
	config := createTestConfig()

	testBody := "test request body"
	req := CreateSlackRequest(http.MethodGet, "text/plain", testBody, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(testBody))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	// Should handle successfully and return 200
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

// TestFormatUsersForSlack_MultipleUsers tests formatting multiple users
func TestFormatUsersForSlack_MultipleUsers(t *testing.T) {
	userIDs := []string{"U87654321", "U11111111", "U22222222"}
	result := formatUsersForSlack(userIDs)

	expected := "<@U87654321>, <@U11111111>, <@U22222222>"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Verify each user is formatted correctly
	if !strings.Contains(result, "<@U87654321>") {
		t.Error("Result should contain <@U87654321>")
	}
	if !strings.Contains(result, "<@U11111111>") {
		t.Error("Result should contain <@U11111111>")
	}
	if !strings.Contains(result, "<@U22222222>") {
		t.Error("Result should contain <@U22222222>")
	}

	// Verify proper comma separation
	if !strings.Contains(result, ", ") {
		t.Error("Result should contain comma-space separation")
	}
}

// TestFormatUsersForSlack_SingleUser tests formatting a single user
func TestFormatUsersForSlack_SingleUser(t *testing.T) {
	userIDs := []string{"U87654321"}
	result := formatUsersForSlack(userIDs)

	expected := "<@U87654321>"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestFormatUsersForSlack_EmptyArray tests formatting an empty array
func TestFormatUsersForSlack_EmptyArray(t *testing.T) {
	userIDs := []string{}
	result := formatUsersForSlack(userIDs)

	if result != "" {
		t.Errorf("Expected empty string for empty array, got %q", result)
	}
}

// TestFormatUsersForSlack_NilArray tests formatting a nil array
func TestFormatUsersForSlack_NilArray(t *testing.T) {
	var userIDs []string
	result := formatUsersForSlack(userIDs)

	if result != "" {
		t.Errorf("Expected empty string for nil array, got %q", result)
	}
}

// TestGiveKudos_UsesFormatUsersForSlack tests that the handler uses the formatting function
func TestGiveKudos_UsesFormatUsersForSlack(t *testing.T) {
	postMessageCalled := false

	config := createTestConfig()
	config.SlackAPI = &MockSlackClient{
		PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			postMessageCalled = true
			return channelID, "1234567890.123456", nil
		},
	}

	payload := ValidInteractionCallbackPayload()
	formData := url.Values{}
	formData.Set("payload", payload)
	body := formData.Encode()

	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	if !postMessageCalled {
		t.Error("Expected PostMessage to be called")
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

// Benchmark tests
func BenchmarkGenerateSlackSignature(b *testing.B) {
	secret := "test-secret"
	body := "test-body-with-some-content"
	timestamp := int64(1234567890)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateSlackSignature(secret, body, timestamp)
	}
}

func BenchmarkHandleKudos_ValidRequest(b *testing.B) {
	config := createTestConfig()
	body := "trigger_id=12345.67890"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
		req.Body = io.NopCloser(bytes.NewReader([]byte(body)))
		req.URL = &url.URL{Path: "/"}
		req.RequestURI = "/"

		rr := httptest.NewRecorder()
		handleKudos(rr, req, config)
	}
}

// Tests for dynamic modal interaction (block_actions)

func TestHandleBlockActions_KudoTypeSelection_EmptyMessage(t *testing.T) {
	viewsUpdateCalled := false
	var capturedViewID, capturedHash string
	var capturedRequest map[string]interface{}

	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "views.update") {
				viewsUpdateCalled = true

				// Capture request body
				bodyBytes, _ := io.ReadAll(req.Body)
				json.Unmarshal(bodyBytes, &capturedRequest)
				capturedViewID = capturedRequest["view_id"].(string)
				capturedHash = capturedRequest["hash"].(string)

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
			}, nil
		},
	}

	// Create block_actions callback
	callback := &slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		View: slack.View{
			ID:   "V12345",
			Hash: "hash123",
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"kudo_message": {
						"kudo_message": {Value: ""}, // Empty message
					},
				},
			},
		},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{
				{
					ActionID: "kudo_type",
					SelectedOption: slack.OptionBlockObject{
						Value: "entrega-excepcional",
					},
				},
			},
		},
	}

	rr := httptest.NewRecorder()
	handleBlockActions(rr, callback, config)

	if !viewsUpdateCalled {
		t.Error("Expected views.update to be called")
	}

	if capturedViewID != "V12345" {
		t.Errorf("Expected view_id V12345, got %s", capturedViewID)
	}

	if capturedHash != "hash123" {
		t.Errorf("Expected hash hash123, got %s", capturedHash)
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", status)
	}
}

func TestHandleBlockActions_KudoTypeSelection_PreservesExistingMessage(t *testing.T) {
	viewsUpdateCalled := false
	var capturedRequest map[string]interface{}

	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "views.update") {
				viewsUpdateCalled = true

				// Capture request body
				bodyBytes, _ := io.ReadAll(req.Body)
				json.Unmarshal(bodyBytes, &capturedRequest)

				// Check that existing message is preserved
				view := capturedRequest["view"].(map[string]interface{})
				blocks := view["blocks"].([]interface{})
				for _, block := range blocks {
					blockMap := block.(map[string]interface{})
					if blockMap["block_id"] == "kudo_message" {
						element := blockMap["element"].(map[string]interface{})
						if initialValue, ok := element["initial_value"].(string); ok {
							if initialValue != "User typed this message" {
								t.Errorf("Expected existing message to be preserved, got: %s", initialValue)
							}
						}
					}
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok": true}`))}, nil
		},
	}

	// Create block_actions callback with existing message
	callback := &slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		View: slack.View{
			ID:   "V12345",
			Hash: "hash123",
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"kudo_message": {
						"kudo_message": {Value: "User typed this message"}, // Existing message
					},
				},
			},
		},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{
				{
					ActionID: "kudo_type",
					SelectedOption: slack.OptionBlockObject{
						Value: "entrega-excepcional",
					},
				},
			},
		},
	}

	rr := httptest.NewRecorder()
	handleBlockActions(rr, callback, config)

	if !viewsUpdateCalled {
		t.Error("Expected views.update to be called")
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", status)
	}
}

func TestHandleBlockActions_NonKudoTypeAction(t *testing.T) {
	viewsUpdateCalled := false

	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "views.update") {
				viewsUpdateCalled = true
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok": true}`))}, nil
		},
	}

	// Create block_actions callback for different action
	callback := &slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		View: slack.View{
			ID:   "V12345",
			Hash: "hash123",
		},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{
				{
					ActionID: "some_other_action", // Not kudo_type
					SelectedOption: slack.OptionBlockObject{
						Value: "some-value",
					},
				},
			},
		},
	}

	rr := httptest.NewRecorder()
	handleBlockActions(rr, callback, config)

	if viewsUpdateCalled {
		t.Error("Expected views.update NOT to be called for non-kudo_type actions")
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", status)
	}
}

func TestHandleBlockActions_EmptyBlockActions(t *testing.T) {
	viewsUpdateCalled := false

	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "views.update") {
				viewsUpdateCalled = true
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok": true}`))}, nil
		},
	}

	callback := &slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		View: slack.View{
			ID:   "V12345",
			Hash: "hash123",
		},
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{}, // Empty actions
		},
	}

	rr := httptest.NewRecorder()
	handleBlockActions(rr, callback, config)

	if viewsUpdateCalled {
		t.Error("Expected views.update NOT to be called for empty block actions")
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", status)
	}
}

func TestUpdateView_Success(t *testing.T) {
	viewsUpdateCalled := false
	var capturedAuthHeader string

	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			viewsUpdateCalled = true
			capturedAuthHeader = req.Header.Get("Authorization")

			if req.Header.Get("Content-Type") != "application/json" {
				t.Error("Expected Content-Type to be application/json")
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		},
	}

	err := updateView("V12345", "hash123", "entrega-excepcional", "Test message", config)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !viewsUpdateCalled {
		t.Error("Expected views.update to be called")
	}

	expectedAuth := "Bearer xoxb-test-token"
	if capturedAuthHeader != expectedAuth {
		t.Errorf("Expected Authorization header %s, got %s", expectedAuth, capturedAuthHeader)
	}
}

func TestUpdateView_SlackAPIError(t *testing.T) {
	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok": false, "error": "invalid_view"}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		},
	}

	err := updateView("V12345", "hash123", "entrega-excepcional", "Test message", config)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid_view") {
		t.Errorf("Expected error to contain 'invalid_view', got: %v", err)
	}
}

func TestUpdateView_HTTPError(t *testing.T) {
	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		},
	}

	err := updateView("V12345", "hash123", "entrega-excepcional", "Test message", config)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("Expected error to contain 'network error', got: %v", err)
	}
}

func TestUpdateView_EmptyMessage(t *testing.T) {
	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			// Check that initial_value is not set for empty message
			bodyBytes, _ := io.ReadAll(req.Body)
			var requestBody map[string]interface{}
			json.Unmarshal(bodyBytes, &requestBody)

			view := requestBody["view"].(map[string]interface{})
			blocks := view["blocks"].([]interface{})
			for _, block := range blocks {
				blockMap := block.(map[string]interface{})
				if blockMap["block_id"] == "kudo_message" {
					element := blockMap["element"].(map[string]interface{})
					if _, hasInitialValue := element["initial_value"]; hasInitialValue {
						t.Error("Expected initial_value NOT to be set for empty message")
					}
				}
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
			}, nil
		},
	}

	err := updateView("V12345", "hash123", "entrega-excepcional", "", config) // Empty message

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestGiveKudos_BlockActionsIntegration(t *testing.T) {
	viewsUpdateCalled := false

	config := createTestConfig()
	config.HTTPClient = &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "views.update") {
				viewsUpdateCalled = true
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok": true}`)),
			}, nil
		},
	}

	// Create block_actions payload
	blockActionsPayload := map[string]interface{}{
		"type": "block_actions",
		"user": map[string]interface{}{"id": "U123"},
		"view": map[string]interface{}{
			"id":   "V12345",
			"hash": "hash123",
			"state": map[string]interface{}{
				"values": map[string]interface{}{
					"kudo_message": map[string]interface{}{
						"kudo_message": map[string]interface{}{
							"value": "",
						},
					},
				},
			},
		},
		"actions": []interface{}{
			map[string]interface{}{
				"action_id": "kudo_type",
				"block_id":  "kudo_type",
				"type":      "static_select",
				"selected_option": map[string]interface{}{
					"value": "entrega-excepcional",
					"text": map[string]interface{}{
						"type": "plain_text",
						"text": "Entrega Excepcional",
					},
				},
			},
		},
	}

	payloadJSON, _ := json.Marshal(blockActionsPayload)

	formData := url.Values{}
	formData.Set("payload", string(payloadJSON))
	body := formData.Encode()

	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()
	handleKudos(rr, req, config)

	if !viewsUpdateCalled {
		t.Error("Expected views.update to be called for block_actions interaction")
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", status)
	}
}

func TestKudoSuggestedMessages_AllTypesPresent(t *testing.T) {
	expectedTypes := []string{"entrega-excepcional", "espirito-de-equipe", "ideia-brilhante", "acima-e-alem", "mestre-em-ensinar", "resolvedor-de-problemas", "atitude-positiva", "crescimento-continuo", "conquista-do-time", "resiliencia"}

	for _, kudoType := range expectedTypes {
		message, exists := kudoSuggestedMessages[kudoType]
		if !exists {
			t.Errorf("Expected kudoSuggestedMessages to have entry for %s", kudoType)
		}
		if message == "" {
			t.Errorf("Expected non-empty message for %s", kudoType)
		}
	}
}

// TestFormatAsSlackQuote_SingleLine tests formatting a single line message
func TestFormatAsSlackQuote_SingleLine(t *testing.T) {
	message := "This is a single line message"
	result := formatAsSlackQuote(message)

	expected := "> This is a single line message"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestFormatAsSlackQuote_MultipleLines tests formatting a multi-line message
func TestFormatAsSlackQuote_MultipleLines(t *testing.T) {
	message := "Inspirador ver sua dedicação em sempre aprender e evoluir!\nPodendo complementar! :D"
	result := formatAsSlackQuote(message)

	expected := "> Inspirador ver sua dedicação em sempre aprender e evoluir!\n> Podendo complementar! :D"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Verify each line starts with "> "
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "> ") {
			t.Errorf("Line %d does not start with '> ': %q", i, line)
		}
	}
}

// TestFormatAsSlackQuote_ThreeLines tests formatting a three-line message
func TestFormatAsSlackQuote_ThreeLines(t *testing.T) {
	message := "Line one\nLine two\nLine three"
	result := formatAsSlackQuote(message)

	expected := "> Line one\n> Line two\n> Line three"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestFormatAsSlackQuote_EmptyString tests formatting an empty string
func TestFormatAsSlackQuote_EmptyString(t *testing.T) {
	message := ""
	result := formatAsSlackQuote(message)

	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

// TestFormatAsSlackQuote_EmptyLines tests formatting a message with empty lines
func TestFormatAsSlackQuote_EmptyLines(t *testing.T) {
	message := "Line one\n\nLine three"
	result := formatAsSlackQuote(message)

	expected := "> Line one\n> \n> Line three"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}

	// Verify all lines start with "> "
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "> ") {
			t.Errorf("Line %d does not start with '> ': %q", i, line)
		}
	}
}

// TestFormatKudosAsBlocks tests the Block Kit message formatting
func TestFormatKudosAsBlocks(t *testing.T) {
	senderID := "U12345678"
	recipientIDs := []string{"U87654321", "U11111111"}
	kudoTypeEmoji := ":zap:"
	kudoTypeText := "Resolvedor(a) de Problemas"
	message := "Sua habilidade de resolver problemas salvou o dia!\n\nMúltiplas linhas aqui."

	blocks := formatKudosAsBlocks(senderID, recipientIDs, kudoTypeEmoji, kudoTypeText, message)

	// Verify number of blocks (header, section, divider, section, section, divider, context)
	expectedBlockCount := 7
	if len(blocks) != expectedBlockCount {
		t.Errorf("Expected %d blocks, got %d", expectedBlockCount, len(blocks))
	}

	// Verify header block
	headerBlock, ok := blocks[0].(*slack.HeaderBlock)
	if !ok {
		t.Errorf("Expected first block to be HeaderBlock, got %T", blocks[0])
	} else {
		if headerBlock.Text.Text != "🎉 Novo Elogio! 🎉" {
			t.Errorf("Expected header text '🎉 Novo Elogio! 🎉', got %q", headerBlock.Text.Text)
		}
	}

	// Verify section with fields (De/Para)
	sectionBlock, ok := blocks[1].(*slack.SectionBlock)
	if !ok {
		t.Errorf("Expected second block to be SectionBlock, got %T", blocks[1])
	} else {
		if len(sectionBlock.Fields) != 2 {
			t.Errorf("Expected 2 fields in section block, got %d", len(sectionBlock.Fields))
		} else {
			if !strings.Contains(sectionBlock.Fields[0].Text, "De:") {
				t.Error("Expected first field to contain 'De:'")
			}
			if !strings.Contains(sectionBlock.Fields[0].Text, senderID) {
				t.Errorf("Expected first field to contain sender ID %s", senderID)
			}
			if !strings.Contains(sectionBlock.Fields[1].Text, "Para:") {
				t.Error("Expected second field to contain 'Para:'")
			}
			if !strings.Contains(sectionBlock.Fields[1].Text, "U87654321") {
				t.Error("Expected second field to contain recipient U87654321")
			}
			if !strings.Contains(sectionBlock.Fields[1].Text, "U11111111") {
				t.Error("Expected second field to contain recipient U11111111")
			}
		}
	}

	// Verify divider
	_, ok = blocks[2].(*slack.DividerBlock)
	if !ok {
		t.Errorf("Expected third block to be DividerBlock, got %T", blocks[2])
	}

	// Verify kudos type section
	kudoTypeBlock, ok := blocks[3].(*slack.SectionBlock)
	if !ok {
		t.Errorf("Expected fourth block to be SectionBlock, got %T", blocks[3])
	} else {
		if !strings.Contains(kudoTypeBlock.Text.Text, kudoTypeEmoji) {
			t.Errorf("Expected kudos type block to contain emoji %s", kudoTypeEmoji)
		}
		if !strings.Contains(kudoTypeBlock.Text.Text, kudoTypeText) {
			t.Errorf("Expected kudos type block to contain text %s", kudoTypeText)
		}
	}

	// Verify message section (quoted)
	messageBlock, ok := blocks[4].(*slack.SectionBlock)
	if !ok {
		t.Errorf("Expected fifth block to be SectionBlock, got %T", blocks[4])
	} else {
		if !strings.HasPrefix(messageBlock.Text.Text, "> ") {
			t.Error("Expected message to be formatted as quote (starting with '> ')")
		}
		if !strings.Contains(messageBlock.Text.Text, "Sua habilidade de resolver problemas salvou o dia!") {
			t.Error("Expected message block to contain the original message content")
		}
	}

	// Verify divider
	_, ok = blocks[5].(*slack.DividerBlock)
	if !ok {
		t.Errorf("Expected sixth block to be DividerBlock, got %T", blocks[5])
	}

	// Verify context (footer)
	contextBlock, ok := blocks[6].(*slack.ContextBlock)
	if !ok {
		t.Errorf("Expected seventh block to be ContextBlock, got %T", blocks[6])
	} else {
		if len(contextBlock.ContextElements.Elements) == 0 {
			t.Error("Expected context block to have elements")
		}
	}
}

// TestFormatKudosAsBlocks_EmptyMessage tests Block Kit formatting with empty message
func TestFormatKudosAsBlocks_EmptyMessage(t *testing.T) {
	senderID := "U12345678"
	recipientIDs := []string{"U87654321"}
	kudoTypeEmoji := ":dart:"
	kudoTypeText := "Entrega Excepcional"
	message := ""

	blocks := formatKudosAsBlocks(senderID, recipientIDs, kudoTypeEmoji, kudoTypeText, message)

	// Should still create all blocks even with empty message
	if len(blocks) != 7 {
		t.Errorf("Expected 7 blocks even with empty message, got %d", len(blocks))
	}

	// Verify message block has empty quoted string
	messageBlock, ok := blocks[4].(*slack.SectionBlock)
	if !ok {
		t.Errorf("Expected fifth block to be SectionBlock, got %T", blocks[4])
	} else {
		if messageBlock.Text.Text != "" {
			t.Errorf("Expected empty message block, got %q", messageBlock.Text.Text)
		}
	}
}

// TestGiveKudos_PostsBlockKitMessage tests that the handler posts Block Kit formatted messages
func TestGiveKudos_PostsBlockKitMessage(t *testing.T) {
	postMessageCalled := false
	var capturedOptions []slack.MsgOption

	config := createTestConfig()
	config.SlackAPI = &MockSlackClient{
		PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			postMessageCalled = true
			capturedOptions = options
			return channelID, "1234567890.123456", nil
		},
	}

	payload := ValidInteractionCallbackPayload()
	formData := url.Values{}
	formData.Set("payload", payload)
	body := formData.Encode()

	req := CreateSlackRequest(http.MethodPost, "application/x-www-form-urlencoded", body, config.SigningSecret)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.URL = &url.URL{Path: "/"}
	req.RequestURI = "/"

	rr := httptest.NewRecorder()

	handleKudos(rr, req, config)

	if !postMessageCalled {
		t.Error("Expected PostMessage to be called")
	}

	// Verify that we're passing options (blocks + fallback text)
	if len(capturedOptions) == 0 {
		t.Error("Expected PostMessage to be called with options")
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}
