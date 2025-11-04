package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
)

// MockHTTPClient for testing HTTP requests
type MockHTTPClientReminder struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClientReminder) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Body:       http.NoBody,
	}, nil
}

func TestHandleReminderButton(t *testing.T) {
	tests := []struct {
		name               string
		callback           *slack.InteractionCallback
		mockHTTPFunc       func(req *http.Request) (*http.Response, error)
		expectedStatusCode int
	}{
		{
			name: "successful button click opens modal",
			callback: &slack.InteractionCallback{
				Type:      slack.InteractionTypeBlockActions,
				TriggerID: "12345.67890.abcdef",
				User: slack.User{
					ID:   "U123456",
					Name: "john.doe",
				},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID: "open_kudos_modal",
							Value:    "open_modal",
						},
					},
				},
			},
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
			callback: &slack.InteractionCallback{
				Type:      slack.InteractionTypeBlockActions,
				TriggerID: "", // Missing
				User: slack.User{
					ID:   "U123456",
					Name: "john.doe",
				},
			},
			mockHTTPFunc: func(req *http.Request) (*http.Response, error) {
				body := `{"ok":true,"view":{"id":"V123456"}}`
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expectedStatusCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClientReminder{
				DoFunc: tt.mockHTTPFunc,
			}

			cfg := &config.Config{
				SlackBotToken:  "xoxb-test-token",
				SlackChannelID: "C123456",
				SigningSecret:  "test-secret",
				HTTPClient:     mockHTTP,
			}

			w := httptest.NewRecorder()
			HandleReminderButton(w, tt.callback, `{"type":"modal","title":{"type":"plain_text","text":"Test"}}`, cfg)

			if w.Code != tt.expectedStatusCode {
				t.Errorf("HandleReminderButton() status = %d, want %d", w.Code, tt.expectedStatusCode)
			}
		})
	}
}
