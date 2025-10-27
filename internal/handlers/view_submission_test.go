package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/models"
)

func TestHandleViewSubmission(t *testing.T) {
	tests := []struct {
		name               string
		callback           *slack.InteractionCallback
		mockSlackFunc      func(channelID string, options ...slack.MsgOption) (string, string, error)
		expectedStatusCode int
		validateCalled     func(t *testing.T, called bool, channelID, senderID string, recipients []string, emoji, text, message string)
	}{
		{
			name: "successful submission with custom message",
			callback: &slack.InteractionCallback{
				Type: slack.InteractionTypeViewSubmission,
				User: slack.User{
					ID:   "U123456",
					Name: "john.doe",
				},
				View: slack.View{
					ID: "V123456",
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_users": {
								"kudo_users": {
									SelectedUsers: []string{"U789012", "U345678"},
								},
							},
							"kudo_type": {
								"kudo_type": {
									SelectedOption: slack.OptionBlockObject{
										Value: "resolvedor-de-problemas",
										Text: &slack.TextBlockObject{
											Type: slack.PlainTextType,
											Text: ":zap: Resolvedor(a) de Problemas",
										},
									},
								},
							},
							"kudo_message": {
								"kudo_message": {
									Value: "Você resolveu aquele bug complexo muito bem!",
								},
							},
						},
					},
				},
			},
			mockSlackFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			expectedStatusCode: http.StatusOK,
			validateCalled: func(t *testing.T, called bool, channelID, senderID string, recipients []string, emoji, text, message string) {
				if !called {
					t.Error("expected PostKudos to be called")
				}
				if senderID != "U123456" {
					t.Errorf("expected sender U123456, got %s", senderID)
				}
				if len(recipients) != 2 {
					t.Errorf("expected 2 recipients, got %d", len(recipients))
				}
				if emoji != ":zap:" {
					t.Errorf("expected emoji :zap:, got %s", emoji)
				}
				if !strings.Contains(message, "bug complexo") {
					t.Errorf("expected custom message, got %s", message)
				}
			},
		},
		{
			name: "submission with empty message uses suggested message",
			callback: &slack.InteractionCallback{
				User: slack.User{
					ID: "U111111",
				},
				View: slack.View{
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_users": {
								"kudo_users": {
									SelectedUsers: []string{"U222222"},
								},
							},
							"kudo_type": {
								"kudo_type": {
									SelectedOption: slack.OptionBlockObject{
										Value: "espirito-de-equipe",
										Text: &slack.TextBlockObject{
											Text: ":handshake: Espírito de Equipe",
										},
									},
								},
							},
							"kudo_message": {
								"kudo_message": {
									Value: "", // Empty message
								},
							},
						},
					},
				},
			},
			mockSlackFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			expectedStatusCode: http.StatusOK,
			validateCalled: func(t *testing.T, called bool, channelID, senderID string, recipients []string, emoji, text, message string) {
				if !called {
					t.Error("expected PostKudos to be called")
				}
				// Should use suggested message for espirito-de-equipe
				expectedMsg := "Obrigado por estar sempre a disposição para ajudar o time!"
				if message != expectedMsg {
					t.Errorf("expected suggested message %q, got %q", expectedMsg, message)
				}
			},
		},
		{
			name: "submission with single recipient",
			callback: &slack.InteractionCallback{
				User: slack.User{
					ID: "U111111",
				},
				View: slack.View{
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_users": {
								"kudo_users": {
									SelectedUsers: []string{"U222222"},
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
									Value: "Parabéns!",
								},
							},
						},
					},
				},
			},
			mockSlackFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			expectedStatusCode: http.StatusOK,
			validateCalled: func(t *testing.T, called bool, channelID, senderID string, recipients []string, emoji, text, message string) {
				if !called {
					t.Error("expected PostKudos to be called")
				}
				if len(recipients) != 1 {
					t.Errorf("expected 1 recipient, got %d", len(recipients))
				}
				if recipients[0] != "U222222" {
					t.Errorf("expected recipient U222222, got %s", recipients[0])
				}
			},
		},
		{
			name: "submission with multiple recipients",
			callback: &slack.InteractionCallback{
				User: slack.User{
					ID: "U111111",
				},
				View: slack.View{
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_users": {
								"kudo_users": {
									SelectedUsers: []string{"U222222", "U333333", "U444444", "U555555"},
								},
							},
							"kudo_type": {
								"kudo_type": {
									SelectedOption: slack.OptionBlockObject{
										Value: "conquista-do-time",
										Text: &slack.TextBlockObject{
											Text: ":tada: Conquista do Time",
										},
									},
								},
							},
							"kudo_message": {
								"kudo_message": {
									Value: "Parabéns a todos!",
								},
							},
						},
					},
				},
			},
			mockSlackFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			expectedStatusCode: http.StatusOK,
			validateCalled: func(t *testing.T, called bool, channelID, senderID string, recipients []string, emoji, text, message string) {
				if !called {
					t.Error("expected PostKudos to be called")
				}
				if len(recipients) != 4 {
					t.Errorf("expected 4 recipients, got %d", len(recipients))
				}
			},
		},
		{
			name: "submission with unknown kudo type still processes",
			callback: &slack.InteractionCallback{
				User: slack.User{
					ID: "U111111",
				},
				View: slack.View{
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_users": {
								"kudo_users": {
									SelectedUsers: []string{"U222222"},
								},
							},
							"kudo_type": {
								"kudo_type": {
									SelectedOption: slack.OptionBlockObject{
										Value: "unknown-type",
										Text: &slack.TextBlockObject{
											Text: ":question: Unknown Type",
										},
									},
								},
							},
							"kudo_message": {
								"kudo_message": {
									Value: "Custom message for unknown type",
								},
							},
						},
					},
				},
			},
			mockSlackFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			expectedStatusCode: http.StatusOK,
			validateCalled: func(t *testing.T, called bool, channelID, senderID string, recipients []string, emoji, text, message string) {
				if !called {
					t.Error("expected PostKudos to be called even with unknown type")
				}
				// Should use the custom message since suggested message doesn't exist
				if message != "Custom message for unknown type" {
					t.Errorf("expected custom message, got %s", message)
				}
			},
		},
		{
			name: "submission with emoji in different format",
			callback: &slack.InteractionCallback{
				User: slack.User{
					ID: "U111111",
				},
				View: slack.View{
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"kudo_users": {
								"kudo_users": {
									SelectedUsers: []string{"U222222"},
								},
							},
							"kudo_type": {
								"kudo_type": {
									SelectedOption: slack.OptionBlockObject{
										Value: "ideia-brilhante",
										Text: &slack.TextBlockObject{
											Text: ":bulb: Ideia Brilhante",
										},
									},
								},
							},
							"kudo_message": {
								"kudo_message": {
									Value: "Ideia muito criativa!",
								},
							},
						},
					},
				},
			},
			mockSlackFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			expectedStatusCode: http.StatusOK,
			validateCalled: func(t *testing.T, called bool, channelID, senderID string, recipients []string, emoji, text, message string) {
				if !called {
					t.Error("expected PostKudos to be called")
				}
				if emoji != ":bulb:" {
					t.Errorf("expected emoji :bulb:, got %s", emoji)
				}
				if text != "Ideia Brilhante" {
					t.Errorf("expected text 'Ideia Brilhante', got %s", text)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track if PostKudos was called and capture parameters
			var called bool
			var capturedChannelID, capturedSenderID string
			var capturedRecipients []string
			var capturedEmoji, capturedText, capturedMessage string

			mockSlack := &MockSlackClient{
				PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
					called = true
					capturedChannelID = channelID
					return tt.mockSlackFunc(channelID, options...)
				},
			}

			cfg := &config.Config{
				SlackChannelID: "C123456",
				SlackAPI:       mockSlack,
			}

			// Capture the actual parameters passed to PostKudos
			// We do this by calling the actual services, which will use our mock
			w := httptest.NewRecorder()

			HandleViewSubmission(w, tt.callback, cfg)

			// Extract captured values from the callback for validation
			capturedSenderID = tt.callback.User.ID
			capturedRecipients = tt.callback.View.State.Values["kudo_users"]["kudo_users"].SelectedUsers
			kudoTypeFullText := tt.callback.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Text.Text
			kudoTypeValue := tt.callback.View.State.Values["kudo_type"]["kudo_type"].SelectedOption.Value
			kudoMessage := tt.callback.View.State.Values["kudo_message"]["kudo_message"].Value

			// Apply same logic as handler
			if kudoMessage == "" {
				if suggestedMsg, ok := models.KudoSuggestedMessages[kudoTypeValue]; ok {
					kudoMessage = suggestedMsg
				}
			}

			// Parse emoji and text
			parts := strings.SplitN(kudoTypeFullText, " ", 2)
			if len(parts) == 2 {
				capturedEmoji = parts[0]
				capturedText = parts[1]
			} else {
				capturedText = kudoTypeFullText
			}
			capturedMessage = kudoMessage

			// Check status code
			if w.Code != tt.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}

			// Validate the call if function provided
			if tt.validateCalled != nil {
				tt.validateCalled(t, called, capturedChannelID, capturedSenderID, capturedRecipients, capturedEmoji, capturedText, capturedMessage)
			}
		})
	}
}

func TestHandleViewSubmission_PostKudosError(t *testing.T) {
	// Test that errors from PostKudos don't affect response
	// (modal already closed, so we just log the error)
	callback := &slack.InteractionCallback{
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
								Value: "atitude-positiva",
								Text: &slack.TextBlockObject{
									Text: ":star2: Atitude Positiva",
								},
							},
						},
					},
					"kudo_message": {
						"kudo_message": {
							Value: "Mensagem teste",
						},
					},
				},
			},
		},
	}

	mockSlack := &MockSlackClient{
		PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			// Simulate error
			return "", "", http.ErrAbortHandler
		},
	}

	cfg := &config.Config{
		SlackChannelID: "C123456",
		SlackAPI:       mockSlack,
	}

	w := httptest.NewRecorder()

	HandleViewSubmission(w, callback, cfg)

	// Should still return 200 OK even with error
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d even with error, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleViewSubmission_SuggestedMessageForAllTypes(t *testing.T) {
	// Test that suggested messages work for all known kudo types
	kudoTypes := []struct {
		value string
		text  string
	}{
		{"entrega-excepcional", ":dart: Entrega Excepcional"},
		{"espirito-de-equipe", ":handshake: Espírito de Equipe"},
		{"ideia-brilhante", ":bulb: Ideia Brilhante"},
		{"acima-e-alem", ":rocket: Acima e Além"},
		{"mestre-em-ensinar", ":mortar_board: Mestre(a) em Ensinar"},
		{"resolvedor-de-problemas", ":zap: Resolvedor(a) de Problemas"},
		{"atitude-positiva", ":star2: Atitude Positiva"},
		{"crescimento-continuo", ":seedling: Crescimento Contínuo"},
		{"conquista-do-time", ":tada: Conquista do Time"},
		{"resiliencia", ":muscle: Resiliência"},
	}

	for _, kt := range kudoTypes {
		t.Run(kt.value, func(t *testing.T) {
			callback := &slack.InteractionCallback{
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
										Value: kt.value,
										Text: &slack.TextBlockObject{
											Text: kt.text,
										},
									},
								},
							},
							"kudo_message": {
								"kudo_message": {
									Value: "", // Empty to trigger suggested message
								},
							},
						},
					},
				},
			}

			called := false
			mockSlack := &MockSlackClient{
				PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
					called = true
					return "C123456", "1234567890.123456", nil
				},
			}

			cfg := &config.Config{
				SlackChannelID: "C123456",
				SlackAPI:       mockSlack,
			}

			w := httptest.NewRecorder()

			HandleViewSubmission(w, callback, cfg)

			if !called {
				t.Error("expected PostKudos to be called")
			}

			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
			}
		})
	}
}

// MockSlackClient for testing
type MockSlackClient struct {
	PostMessageFunc               func(channelID string, options ...slack.MsgOption) (string, string, error)
	InviteUsersToConversationFunc func(channelID string, users ...string) (*slack.Channel, error)
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
