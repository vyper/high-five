package services

import (
	"errors"
	"testing"

	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
)

// MockSlackClient is a mock implementation of config.SlackClient
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

func TestPostKudos(t *testing.T) {
	tests := []struct {
		name          string
		senderID      string
		recipientIDs  []string
		kudoTypeEmoji string
		kudoTypeText  string
		message       string
		mockFunc      func(channelID string, options ...slack.MsgOption) (string, string, error)
		wantErr       bool
		errContains   string
	}{
		{
			name:          "successful kudos post",
			senderID:      "U123456",
			recipientIDs:  []string{"U789012", "U345678"},
			kudoTypeEmoji: ":zap:",
			kudoTypeText:  "Resolvedor(a) de Problemas",
			message:       "Obrigado por resolver aquele bug complexo!",
			mockFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				// Verify channel ID
				if channelID != "C123456" {
					t.Errorf("expected channel C123456, got %s", channelID)
				}
				return "C123456", "1234567890.123456", nil
			},
			wantErr: false,
		},
		{
			name:          "single recipient",
			senderID:      "U111111",
			recipientIDs:  []string{"U222222"},
			kudoTypeEmoji: ":star:",
			kudoTypeText:  "Entrega Excepcional",
			message:       "Excelente trabalho!",
			mockFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			wantErr: false,
		},
		{
			name:          "empty message",
			senderID:      "U111111",
			recipientIDs:  []string{"U222222"},
			kudoTypeEmoji: ":rocket:",
			kudoTypeText:  "Acima e Além",
			message:       "",
			mockFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			wantErr: false,
		},
		{
			name:          "Slack API error",
			senderID:      "U123456",
			recipientIDs:  []string{"U789012"},
			kudoTypeEmoji: ":tada:",
			kudoTypeText:  "Conquista do Time",
			message:       "Parabéns pela conquista!",
			mockFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "", "", errors.New("channel_not_found")
			},
			wantErr:     true,
			errContains: "error posting message",
		},
		{
			name:          "multiple recipients",
			senderID:      "U123456",
			recipientIDs:  []string{"U111111", "U222222", "U333333"},
			kudoTypeEmoji: ":muscle:",
			kudoTypeText:  "Espírito de Equipe",
			message:       "Vocês são incríveis trabalhando juntos!",
			mockFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "C123456", "1234567890.123456", nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSlack := &MockSlackClient{
				PostMessageFunc: tt.mockFunc,
			}

			cfg := &config.Config{
				SlackChannelID: "C123456",
				SlackAPI:       mockSlack,
			}

			err := PostKudos(
				tt.senderID,
				tt.recipientIDs,
				tt.kudoTypeEmoji,
				tt.kudoTypeText,
				tt.message,
				cfg,
			)

			if tt.wantErr {
				if err == nil {
					t.Errorf("PostKudos() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("PostKudos() error = %v, want error containing %s", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("PostKudos() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParseKudoTypeText(t *testing.T) {
	tests := []struct {
		name              string
		kudoTypeFullText  string
		expectedEmoji     string
		expectedText      string
	}{
		{
			name:             "standard emoji and text",
			kudoTypeFullText: ":zap: Resolvedor(a) de Problemas",
			expectedEmoji:    ":zap:",
			expectedText:     "Resolvedor(a) de Problemas",
		},
		{
			name:             "emoji with multiple words",
			kudoTypeFullText: ":star: Entrega Excepcional",
			expectedEmoji:    ":star:",
			expectedText:     "Entrega Excepcional",
		},
		{
			name:             "emoji with text containing parentheses",
			kudoTypeFullText: ":bulb: Ideia Brilhante (muito criativo)",
			expectedEmoji:    ":bulb:",
			expectedText:     "Ideia Brilhante (muito criativo)",
		},
		{
			name:             "no space - text only",
			kudoTypeFullText: "TextoSemEmoji",
			expectedEmoji:    "",
			expectedText:     "TextoSemEmoji",
		},
		{
			name:             "empty string",
			kudoTypeFullText: "",
			expectedEmoji:    "",
			expectedText:     "",
		},
		{
			name:             "only emoji no text",
			kudoTypeFullText: ":tada:",
			expectedEmoji:    "",
			expectedText:     ":tada:",
		},
		{
			name:             "emoji with space at end",
			kudoTypeFullText: ":rocket: ",
			expectedEmoji:    ":rocket:",
			expectedText:     "",
		},
		{
			name:             "multiple spaces",
			kudoTypeFullText: ":heart: Muito   Obrigado",
			expectedEmoji:    ":heart:",
			expectedText:     "Muito   Obrigado",
		},
		{
			name:             "unicode emoji and text",
			kudoTypeFullText: "🎉 Celebração",
			expectedEmoji:    "🎉",
			expectedText:     "Celebração",
		},
		{
			name:             "long text after emoji",
			kudoTypeFullText: ":trophy: Esta é uma descrição muito longa de um tipo de elogio que alguém pode receber",
			expectedEmoji:    ":trophy:",
			expectedText:     "Esta é uma descrição muito longa de um tipo de elogio que alguém pode receber",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emoji, text := ParseKudoTypeText(tt.kudoTypeFullText)

			if emoji != tt.expectedEmoji {
				t.Errorf("ParseKudoTypeText() emoji = %q, want %q", emoji, tt.expectedEmoji)
			}

			if text != tt.expectedText {
				t.Errorf("ParseKudoTypeText() text = %q, want %q", text, tt.expectedText)
			}
		})
	}
}

func TestPostKudos_FallbackText(t *testing.T) {
	var capturedOptions []slack.MsgOption

	mockSlack := &MockSlackClient{
		PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			capturedOptions = options
			return "C123456", "1234567890.123456", nil
		},
	}

	cfg := &config.Config{
		SlackChannelID: "C123456",
		SlackAPI:       mockSlack,
	}

	err := PostKudos(
		"U111111",
		[]string{"U222222", "U333333"},
		":zap:",
		"Resolvedor(a) de Problemas",
		"Ótimo trabalho!",
		cfg,
	)

	if err != nil {
		t.Fatalf("PostKudos() unexpected error = %v", err)
	}

	if len(capturedOptions) == 0 {
		t.Fatal("PostKudos() should have passed MsgOptions to PostMessage")
	}

	// Verify that the options include both blocks and text
	// We can't easily inspect the MsgOption values directly, but we can verify they were passed
	// This is more of a smoke test to ensure the function constructs the message correctly
}

func TestInviteUsersToChannel(t *testing.T) {
	tests := []struct {
		name         string
		recipientIDs []string
		mockFunc     func(channelID string, users ...string) (*slack.Channel, error)
		expectCalls  int
	}{
		{
			name:         "successfully invite single user",
			recipientIDs: []string{"U123456"},
			mockFunc: func(channelID string, users ...string) (*slack.Channel, error) {
				if channelID != "C123456" {
					t.Errorf("expected channel C123456, got %s", channelID)
				}
				if len(users) != 1 || users[0] != "U123456" {
					t.Errorf("expected user U123456, got %v", users)
				}
				return &slack.Channel{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: channelID}}}, nil
			},
			expectCalls: 1,
		},
		{
			name:         "successfully invite multiple users",
			recipientIDs: []string{"U111111", "U222222", "U333333"},
			mockFunc: func(channelID string, users ...string) (*slack.Channel, error) {
				return &slack.Channel{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: channelID}}}, nil
			},
			expectCalls: 3,
		},
		{
			name:         "ignore already_in_channel error",
			recipientIDs: []string{"U123456"},
			mockFunc: func(channelID string, users ...string) (*slack.Channel, error) {
				return nil, errors.New("already_in_channel")
			},
			expectCalls: 1,
		},
		{
			name:         "log other errors but continue",
			recipientIDs: []string{"U123456", "U789012"},
			mockFunc: func(channelID string, users ...string) (*slack.Channel, error) {
				if users[0] == "U123456" {
					return nil, errors.New("some_other_error")
				}
				return &slack.Channel{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: channelID}}}, nil
			},
			expectCalls: 2,
		},
		{
			name:         "empty recipient list",
			recipientIDs: []string{},
			mockFunc: func(channelID string, users ...string) (*slack.Channel, error) {
				t.Error("should not be called with empty recipient list")
				return nil, nil
			},
			expectCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mockSlack := &MockSlackClient{
				InviteUsersToConversationFunc: func(channelID string, users ...string) (*slack.Channel, error) {
					callCount++
					return tt.mockFunc(channelID, users...)
				},
			}

			cfg := &config.Config{
				SlackChannelID: "C123456",
				SlackAPI:       mockSlack,
			}

			// This function doesn't return errors, it only logs them
			InviteUsersToChannel(tt.recipientIDs, cfg)

			if callCount != tt.expectCalls {
				t.Errorf("expected %d calls to InviteUsersToConversation, got %d", tt.expectCalls, callCount)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}