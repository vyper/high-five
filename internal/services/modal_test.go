package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/vyper/my-matter/internal/config"
)

// MockHTTPClient is a mock implementation of config.HTTPClient
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
		Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
	}, nil
}

func TestOpenModal(t *testing.T) {
	validTemplate := `{
		"view": {
			"type": "modal",
			"blocks": []
		}
	}`

	tests := []struct {
		name         string
		triggerID    string
		viewTemplate string
		mockFunc     func(req *http.Request) (*http.Response, error)
		wantErr      bool
		errContains  string
	}{
		{
			name:         "successful modal open",
			triggerID:    "12345.67890",
			viewTemplate: validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				// Verify request method and URL
				if req.Method != "POST" {
					t.Errorf("expected POST method, got %s", req.Method)
				}
				if req.URL.String() != "https://slack.com/api/views.open" {
					t.Errorf("expected views.open URL, got %s", req.URL.String())
				}

				// Verify headers
				if req.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", req.Header.Get("Content-Type"))
				}
				if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
					t.Errorf("expected Authorization header with Bearer token")
				}

				// Verify body contains trigger_id
				body, _ := io.ReadAll(req.Body)
				var payload map[string]interface{}
				json.Unmarshal(body, &payload)
				if payload["trigger_id"] != "12345.67890" {
					t.Errorf("expected trigger_id in payload")
				}

				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true,"view":{"id":"V123"}}`)),
				}, nil
			},
			wantErr: false,
		},
		{
			name:         "invalid JSON template",
			triggerID:    "12345.67890",
			viewTemplate: `{invalid json`,
			mockFunc:     nil,
			wantErr:      true,
			errContains:  "error parsing view template",
		},
		{
			name:         "HTTP request error",
			triggerID:    "12345.67890",
			viewTemplate: validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("connection timeout")
			},
			wantErr:     true,
			errContains: "error making POST request",
		},
		{
			name:         "Slack API error response",
			triggerID:    "12345.67890",
			viewTemplate: validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString(`{"ok":false,"error":"invalid_trigger"}`)),
				}, nil
			},
			wantErr: false, // OpenModal doesn't check for ok:false, it just logs
		},
		{
			name:         "empty trigger ID",
			triggerID:    "",
			viewTemplate: validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
				}, nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClient{
				DoFunc: tt.mockFunc,
			}

			cfg := &config.Config{
				SlackBotToken: "xoxb-test-token",
				HTTPClient:    mockHTTP,
			}

			err := OpenModal(tt.triggerID, tt.viewTemplate, cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("OpenModal() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("OpenModal() error = %v, want error containing %s", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("OpenModal() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestUpdateModal(t *testing.T) {
	validTemplate := `{
		"view": {
			"type": "modal",
			"blocks": [
				{
					"type": "input",
					"block_id": "kudo_type",
					"element": {
						"type": "static_select",
						"action_id": "kudo_type"
					}
				},
				{
					"type": "input",
					"block_id": "kudo_message",
					"element": {
						"type": "plain_text_input",
						"action_id": "kudo_message"
					}
				}
			]
		}
	}`

	templateWithDescription := `{
		"view": {
			"type": "modal",
			"blocks": [
				{
					"type": "input",
					"block_id": "kudo_type"
				},
				{
					"type": "context",
					"block_id": "kudo_description",
					"elements": [{"type": "mrkdwn", "text": "old description"}]
				},
				{
					"type": "input",
					"block_id": "kudo_message",
					"element": {}
				}
			]
		}
	}`

	tests := []struct {
		name             string
		viewID           string
		hash             string
		selectedKudoType string
		messageValue     string
		viewTemplate     string
		mockFunc         func(req *http.Request) (*http.Response, error)
		wantErr          bool
		errContains      string
		checkBlocks      bool
		expectedBlocks   int
	}{
		{
			name:             "successful modal update with new description block",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "resolvedor-de-problemas",
			messageValue:     "",
			viewTemplate:     validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				// Verify request
				if req.Method != "POST" {
					t.Errorf("expected POST method, got %s", req.Method)
				}
				if req.URL.String() != "https://slack.com/api/views.update" {
					t.Errorf("expected views.update URL, got %s", req.URL.String())
				}

				// Verify payload structure
				body, _ := io.ReadAll(req.Body)
				var payload map[string]interface{}
				json.Unmarshal(body, &payload)

				if payload["view_id"] != "V123456" {
					t.Errorf("expected view_id V123456")
				}
				if payload["hash"] != "hash123" {
					t.Errorf("expected hash hash123")
				}

				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
				}, nil
			},
			wantErr:        false,
			checkBlocks:    true,
			expectedBlocks: 3, // Original 2 + new description block
		},
		{
			name:             "update existing description block",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "entrega-excepcional",
			messageValue:     "Mensagem existente",
			viewTemplate:     templateWithDescription,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				// Read and parse the body
				body, _ := io.ReadAll(req.Body)
				var payload map[string]interface{}
				json.Unmarshal(body, &payload)

				// Verify that message value was preserved
				view := payload["view"].(map[string]interface{})
				blocks := view["blocks"].([]interface{})

				// Should still have 3 blocks
				if len(blocks) != 3 {
					t.Errorf("expected 3 blocks, got %d", len(blocks))
				}

				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
				}, nil
			},
			wantErr:        false,
			checkBlocks:    true,
			expectedBlocks: 3,
		},
		{
			name:             "invalid JSON template",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "ideia-brilhante",
			messageValue:     "",
			viewTemplate:     `{invalid json`,
			mockFunc:         nil,
			wantErr:          true,
			errContains:      "error parsing view template",
		},
		{
			name:             "invalid view structure",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "acima-e-alem",
			messageValue:     "",
			viewTemplate:     `{"no_view_key": {}}`,
			mockFunc:         nil,
			wantErr:          true,
			errContains:      "invalid view structure",
		},
		{
			name:             "invalid blocks structure",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "mestre-em-ensinar",
			messageValue:     "",
			viewTemplate:     `{"view": {"blocks": "not an array"}}`,
			mockFunc:         nil,
			wantErr:          true,
			errContains:      "invalid blocks structure",
		},
		{
			name:             "HTTP request error",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "atitude-positiva",
			messageValue:     "",
			viewTemplate:     validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("network error")
			},
			wantErr:     true,
			errContains: "error making views.update request",
		},
		{
			name:             "Slack API returns error",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "crescimento-continuo",
			messageValue:     "",
			viewTemplate:     validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString(`{"ok":false,"error":"view_not_found"}`)),
				}, nil
			},
			wantErr:     true,
			errContains: "slack API error",
		},
		{
			name:             "unknown kudo type uses default description",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "unknown-type",
			messageValue:     "",
			viewTemplate:     validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				// Verify default description is used
				body, _ := io.ReadAll(req.Body)
				var payload map[string]interface{}
				json.Unmarshal(body, &payload)

				view := payload["view"].(map[string]interface{})
				blocks := view["blocks"].([]interface{})

				// Find the description block
				for _, block := range blocks {
					blockMap := block.(map[string]interface{})
					if blockMap["block_id"] == "kudo_description" {
						elements := blockMap["elements"].([]interface{})
						element := elements[0].(map[string]interface{})
						text := element["text"].(string)
						if !strings.Contains(text, "Tipo de elogio selecionado") {
							t.Errorf("expected default description")
						}
					}
				}

				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
				}, nil
			},
			wantErr: false,
		},
		{
			name:             "preserves message value when provided",
			viewID:           "V123456",
			hash:             "hash123",
			selectedKudoType: "conquista-do-time",
			messageValue:     "Esta é uma mensagem importante",
			viewTemplate:     validTemplate,
			mockFunc: func(req *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(req.Body)
				var payload map[string]interface{}
				json.Unmarshal(body, &payload)

				view := payload["view"].(map[string]interface{})
				blocks := view["blocks"].([]interface{})

				// Find the message block and verify initial_value
				for _, block := range blocks {
					blockMap := block.(map[string]interface{})
					if blockMap["block_id"] == "kudo_message" {
						element := blockMap["element"].(map[string]interface{})
						if element["initial_value"] != "Esta é uma mensagem importante" {
							t.Errorf("expected message value to be preserved")
						}
					}
				}

				return &http.Response{
					StatusCode: 200,
					Status:     "200 OK",
					Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
				}, nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHTTP := &MockHTTPClient{
				DoFunc: tt.mockFunc,
			}

			cfg := &config.Config{
				SlackBotToken: "xoxb-test-token",
				HTTPClient:    mockHTTP,
			}

			err := UpdateModal(
				tt.viewID,
				tt.hash,
				tt.selectedKudoType,
				tt.messageValue,
				tt.viewTemplate,
				cfg,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateModal() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("UpdateModal() error = %v, want error containing %s", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("UpdateModal() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestUpdateModal_DescriptionBlockInsertion(t *testing.T) {
	// Test that description block is inserted after kudo_type block
	template := `{
		"view": {
			"blocks": [
				{"block_id": "kudo_users"},
				{"block_id": "kudo_type"},
				{"block_id": "kudo_message", "element": {}}
			]
		}
	}`

	mockHTTP := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)

			view := payload["view"].(map[string]interface{})
			blocks := view["blocks"].([]interface{})

			// Should have 4 blocks now (original 3 + description)
			if len(blocks) != 4 {
				t.Errorf("expected 4 blocks after insertion, got %d", len(blocks))
			}

			// Description should be at index 2 (after kudo_type)
			block2 := blocks[2].(map[string]interface{})
			if block2["block_id"] != "kudo_description" {
				t.Errorf("expected kudo_description at index 2, got %s", block2["block_id"])
			}

			// Message should now be at index 3
			block3 := blocks[3].(map[string]interface{})
			if block3["block_id"] != "kudo_message" {
				t.Errorf("expected kudo_message at index 3, got %s", block3["block_id"])
			}

			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
			}, nil
		},
	}

	cfg := &config.Config{
		SlackBotToken: "xoxb-test-token",
		HTTPClient:    mockHTTP,
	}

	err := UpdateModal("V123", "hash123", "resolvedor-de-problemas", "", template, cfg)
	if err != nil {
		t.Errorf("UpdateModal() unexpected error = %v", err)
	}
}
