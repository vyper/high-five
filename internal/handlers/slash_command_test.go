package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/vyper/my-matter/internal/config"
)

func TestHandleSlashCommand(t *testing.T) {
	validTemplate := `{
		"view": {
			"type": "modal",
			"blocks": []
		}
	}`

	tests := []struct {
		name               string
		formValues         url.Values
		viewTemplate       string
		mockHTTPFunc       func(req *http.Request) (*http.Response, error)
		expectedStatusCode int
		expectedBodyPart   string
	}{
		{
			name: "successful slash command with valid trigger_id",
			formValues: url.Values{
				"trigger_id": []string{"12345.67890.abcdef"},
				"user_id":    []string{"U123456"},
				"command":    []string{"/elogie"},
			},
			viewTemplate: validTemplate,
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true,"view":{"id":"V123456"}}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "missing trigger_id returns bad request",
			formValues: url.Values{
				"user_id": []string{"U123456"},
				"command": []string{"/elogie"},
			},
			viewTemplate: validTemplate,
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedBodyPart:   "Missing trigger_id",
		},
		{
			name: "empty trigger_id returns bad request",
			formValues: url.Values{
				"trigger_id": []string{""},
				"user_id":    []string{"U123456"},
				"command":    []string{"/elogie"},
			},
			viewTemplate: validTemplate,
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedBodyPart:   "Missing trigger_id",
		},
		{
			name: "OpenModal error returns internal server error",
			formValues: url.Values{
				"trigger_id": []string{"12345.67890.abcdef"},
				"user_id":    []string{"U123456"},
			},
			viewTemplate: `{invalid json`, // Invalid template will cause error
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedBodyPart:   "Internal Server Error",
		},
		{
			name: "trigger_id with special characters",
			formValues: url.Values{
				"trigger_id": []string{"12345.67890.abc-def_ghi"},
				"user_id":    []string{"U123456"},
			},
			viewTemplate: validTemplate,
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true,"view":{"id":"V123456"}}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "trigger_id with whitespace is preserved",
			formValues: url.Values{
				"trigger_id": []string{"12345.67890.abc def"},
				"user_id":    []string{"U123456"},
			},
			viewTemplate: validTemplate,
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true,"view":{"id":"V123456"}}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "very long trigger_id is accepted",
			formValues: url.Values{
				"trigger_id": []string{strings.Repeat("a", 500)},
				"user_id":    []string{"U123456"},
			},
			viewTemplate: validTemplate,
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true,"view":{"id":"V123456"}}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "additional form values don't interfere",
			formValues: url.Values{
				"trigger_id":  []string{"12345.67890.abcdef"},
				"user_id":     []string{"U123456"},
				"user_name":   []string{"john.doe"},
				"command":     []string{"/elogie"},
				"text":        []string{""},
				"response_url": []string{"https://hooks.slack.com/commands/123/456"},
			},
			viewTemplate: validTemplate,
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true,"view":{"id":"V123456"}}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP client
			mockHTTP := &MockHTTPClient{
				DoFunc: tt.mockHTTPFunc,
			}

			cfg := &config.Config{
				SlackBotToken: "xoxb-test-token",
				HTTPClient:    mockHTTP,
			}

			// Create request with form values
			req := httptest.NewRequest(http.MethodPost, "/slack/command", nil)
			req.Form = tt.formValues

			// Create response recorder
			w := httptest.NewRecorder()

			// Call the handler
			HandleSlashCommand(w, req, tt.viewTemplate, cfg)

			// Check status code
			if w.Code != tt.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}

			// Check response body if specified
			if tt.expectedBodyPart != "" {
				body := w.Body.String()
				if !strings.Contains(body, tt.expectedBodyPart) {
					t.Errorf("expected body to contain %q, got %q", tt.expectedBodyPart, body)
				}
			}
		})
	}
}

func TestHandleSlashCommand_HTTPMethod(t *testing.T) {
	// Test that the handler works regardless of HTTP method
	// (form values are what matter)
	validTemplate := `{"view":{"type":"modal"}}`

	formValues := url.Values{
		"trigger_id": []string{"12345.67890.abcdef"},
	}

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

	cfg := &config.Config{
		SlackBotToken: "xoxb-test-token",
		HTTPClient:    mockHTTP,
	}

	methods := []string{http.MethodPost, http.MethodGet}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/slack/command", nil)
			req.Form = formValues

			w := httptest.NewRecorder()

			HandleSlashCommand(w, req, validTemplate, cfg)

			if w.Code != http.StatusOK {
				t.Errorf("expected status %d for method %s, got %d", http.StatusOK, method, w.Code)
			}
		})
	}
}

func TestHandleSlashCommand_OpenModalCalled(t *testing.T) {
	// Verify that OpenModal is called with correct parameters
	expectedTriggerID := "12345.67890.trigger"
	validTemplate := `{"view":{"type":"modal","blocks":[]}}`

	mockHTTP := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			// Verify the request is going to views.open
			if !strings.Contains(req.URL.String(), "views.open") {
				t.Errorf("expected request to views.open, got %s", req.URL.String())
			}

			// Read and verify the body contains trigger_id
			bodyBytes, _ := io.ReadAll(req.Body)
			bodyString := string(bodyBytes)

			if !strings.Contains(bodyString, expectedTriggerID) {
				t.Errorf("expected body to contain trigger_id %s, got %s", expectedTriggerID, bodyString)
			}

			body := `{"ok":true,"view":{"id":"V123456"}}`
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}

	cfg := &config.Config{
		SlackBotToken: "xoxb-test-token",
		HTTPClient:    mockHTTP,
	}

	formValues := url.Values{
		"trigger_id": []string{expectedTriggerID},
	}

	req := httptest.NewRequest(http.MethodPost, "/slack/command", nil)
	req.Form = formValues

	w := httptest.NewRecorder()

	HandleSlashCommand(w, req, validTemplate, cfg)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleSlashCommand_LogsErrors(t *testing.T) {
	// Test that errors are logged (we can't easily capture logs, but we verify behavior)
	tests := []struct {
		name         string
		formValues   url.Values
		viewTemplate string
		shouldError  bool
	}{
		{
			name: "missing trigger_id logs error",
			formValues: url.Values{
				"user_id": []string{"U123"},
			},
			viewTemplate: `{"view":{}}`,
			shouldError:  true,
		},
		{
			name: "invalid template logs error",
			formValues: url.Values{
				"trigger_id": []string{"12345.67890"},
			},
			viewTemplate: `{invalid`,
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			cfg := &config.Config{
				SlackBotToken: "xoxb-test-token",
				HTTPClient:    mockHTTP,
			}

			req := httptest.NewRequest(http.MethodPost, "/slack/command", nil)
			req.Form = tt.formValues

			w := httptest.NewRecorder()

			HandleSlashCommand(w, req, tt.viewTemplate, cfg)

			// Verify that error status codes are returned
			if tt.shouldError && w.Code == http.StatusOK {
				t.Errorf("expected error status code, got %d", w.Code)
			}
		})
	}
}

func TestHandleSlashCommand_MultipleFormValuesForSameKey(t *testing.T) {
	// Test behavior when form has multiple values for trigger_id (edge case)
	// FormValue returns the first value
	formValues := url.Values{
		"trigger_id": []string{"first-trigger", "second-trigger"},
	}

	validTemplate := `{"view":{"type":"modal"}}`

	callCount := 0
	mockHTTP := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			callCount++
			// Verify only one call is made
			if callCount > 1 {
				t.Error("OpenModal should only be called once")
			}

			bodyBytes, _ := io.ReadAll(req.Body)
			bodyString := string(bodyBytes)

			// Should use the first value
			if !strings.Contains(bodyString, "first-trigger") {
				t.Errorf("expected first trigger_id to be used")
			}

			body := `{"ok":true}`
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		},
	}

	cfg := &config.Config{
		SlackBotToken: "xoxb-test-token",
		HTTPClient:    mockHTTP,
	}

	req := httptest.NewRequest(http.MethodPost, "/slack/command", nil)
	req.Form = formValues

	w := httptest.NewRecorder()

	HandleSlashCommand(w, req, validTemplate, cfg)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}
