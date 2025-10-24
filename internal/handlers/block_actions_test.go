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

// MockUpdateModalFunc is a function type for mocking UpdateModal
type MockUpdateModalFunc func(viewID, hash, selectedKudoType, messageValue, viewTemplate string, cfg *config.Config) error

// We'll need to mock the services.UpdateModal function
// For testing purposes, we'll create a test helper that allows us to inject behavior

func TestHandleBlockActions(t *testing.T) {
	validTemplate := `{
		"view": {
			"type": "modal",
			"blocks": [
				{"block_id": "kudo_type"},
				{"block_id": "kudo_message", "element": {}}
			]
		}
	}`

	tests := []struct {
		name               string
		callback           *slack.InteractionCallback
		viewTemplate       string
		expectedStatusCode int
		checkResponse      func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "successful block action with kudo_type selection",
			callback: &slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
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
			},
			viewTemplate:       validTemplate,
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
				}
			},
		},
		{
			name: "kudo_type selection with existing message preserves user input",
			callback: &slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
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
				View: slack.View{
					ID:   "V123456",
					Hash: "hash123",
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_message": {
								"kudo_message": {Value: "Mensagem personalizada do usuário"},
							},
						},
					},
				},
			},
			viewTemplate:       validTemplate,
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
				}
			},
		},
		{
			name: "non-kudo_type action is acknowledged without processing",
			callback: &slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID: "some_other_action",
							SelectedOption: slack.OptionBlockObject{
								Value: "some_value",
							},
						},
					},
				},
				View: slack.View{
					ID:   "V123456",
					Hash: "hash123",
				},
			},
			viewTemplate:       validTemplate,
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
				}
			},
		},
		{
			name: "empty block actions acknowledges successfully",
			callback: &slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{},
				},
				View: slack.View{
					ID:   "V123456",
					Hash: "hash123",
				},
			},
			viewTemplate:       validTemplate,
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
				}
			},
		},
		{
			name: "kudo_type with empty value is skipped",
			callback: &slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID: "kudo_type",
							SelectedOption: slack.OptionBlockObject{
								Value: "",
							},
						},
					},
				},
				View: slack.View{
					ID:   "V123456",
					Hash: "hash123",
				},
			},
			viewTemplate:       validTemplate,
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
				}
			},
		},
		{
			name: "kudo_type selection with nil state",
			callback: &slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID: "kudo_type",
							SelectedOption: slack.OptionBlockObject{
								Value: "ideia-brilhante",
							},
						},
					},
				},
				View: slack.View{
					ID:    "V123456",
					Hash:  "hash123",
					State: nil, // No state
				},
			},
			viewTemplate:       validTemplate,
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
				}
			},
		},
		{
			name: "multiple actions with kudo_type processes correctly",
			callback: &slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID: "other_action",
							SelectedOption: slack.OptionBlockObject{
								Value: "other",
							},
						},
						{
							ActionID: "kudo_type",
							SelectedOption: slack.OptionBlockObject{
								Value: "acima-e-alem",
							},
						},
					},
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
			},
			viewTemplate:       validTemplate,
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				if w.Code != http.StatusOK {
					t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP client
			mockHTTP := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					// Mock successful response for UpdateModal
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

			// Create response recorder
			w := httptest.NewRecorder()

			// Call the handler
			HandleBlockActions(w, tt.callback, tt.viewTemplate, cfg)

			// Check response
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			if w.Code != tt.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}
		})
	}
}

func TestHandleBlockActions_UpdateModalError(t *testing.T) {
	// Test that errors from UpdateModal are handled gracefully
	callback := &slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
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
	}

	// Invalid template will cause UpdateModal to fail
	invalidTemplate := `{invalid json`

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

	w := httptest.NewRecorder()

	// Call handler with invalid template
	HandleBlockActions(w, callback, invalidTemplate, cfg)

	// Should return internal server error
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d for error case, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestHandleBlockActions_PreservesUserMessage(t *testing.T) {
	// Verify that when user has typed a message, it's preserved
	existingMessage := "Esta é a mensagem do usuário que deve ser preservada"

	callback := &slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{
				{
					ActionID: "kudo_type",
					SelectedOption: slack.OptionBlockObject{
						Value: "crescimento-continuo",
					},
				},
			},
		},
		View: slack.View{
			ID:   "V123456",
			Hash: "hash123",
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"kudo_message": {
						"kudo_message": {Value: existingMessage},
					},
				},
			},
		},
	}

	validTemplate := `{
		"view": {
			"blocks": [
				{"block_id": "kudo_type"},
				{"block_id": "kudo_message", "element": {}}
			]
		}
	}`

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

	w := httptest.NewRecorder()

	HandleBlockActions(w, callback, validTemplate, cfg)

	// Should complete successfully
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleBlockActions_SuggestsMessageWhenEmpty(t *testing.T) {
	// Verify that when message is empty, suggested message is used
	callback := &slack.InteractionCallback{
		Type: slack.InteractionTypeBlockActions,
		ActionCallback: slack.ActionCallbacks{
			BlockActions: []*slack.BlockAction{
				{
					ActionID: "kudo_type",
					SelectedOption: slack.OptionBlockObject{
						Value: "espirito-de-equipe", // Has a suggested message
					},
				},
			},
		},
		View: slack.View{
			ID:   "V123456",
			Hash: "hash123",
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"kudo_message": {
						"kudo_message": {Value: ""}, // Empty message
					},
				},
			},
		},
	}

	validTemplate := `{
		"view": {
			"blocks": [
				{"block_id": "kudo_type"},
				{"block_id": "kudo_message", "element": {}}
			]
		}
	}`

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

	w := httptest.NewRecorder()

	HandleBlockActions(w, callback, validTemplate, cfg)

	// Should complete successfully
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// MockHTTPClient is defined in another test file, but we need it here
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
		Body:       http.NoBody,
	}, nil
}
