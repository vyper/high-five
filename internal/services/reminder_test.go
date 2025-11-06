package services

import (
	"errors"
	"testing"

	"github.com/slack-go/slack"
)

// ExtendedMockSlackClient extends the MockSlackClient with reminder-specific methods
type ExtendedMockSlackClient struct {
	MockSlackClient
	GetUsersInConversationFunc func(params *slack.GetUsersInConversationParameters) ([]string, string, error)
	GetUserInfoFunc            func(user string) (*slack.User, error)
}

func (m *ExtendedMockSlackClient) GetUsersInConversation(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
	if m.GetUsersInConversationFunc != nil {
		return m.GetUsersInConversationFunc(params)
	}
	return []string{"U123456", "U789012"}, "", nil
}

func (m *ExtendedMockSlackClient) GetUserInfo(user string) (*slack.User, error) {
	if m.GetUserInfoFunc != nil {
		return m.GetUserInfoFunc(user)
	}
	return &slack.User{
		ID:      user,
		IsBot:   false,
		Deleted: false,
	}, nil
}

func TestGetChannelMembers(t *testing.T) {
	tests := []struct {
		name                string
		mockUsersFunc       func(params *slack.GetUsersInConversationParameters) ([]string, string, error)
		mockUserInfoFunc    func(user string) (*slack.User, error)
		expectedMembers     []string
		expectedMemberCount int
		wantErr             bool
		errContains         string
	}{
		{
			name: "successful single page",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				return []string{"U111111", "U222222", "U333333"}, "", nil
			},
			mockUserInfoFunc: func(user string) (*slack.User, error) {
				return &slack.User{
					ID:      user,
					IsBot:   false,
					Deleted: false,
				}, nil
			},
			expectedMemberCount: 3,
			wantErr:             false,
		},
		{
			name: "pagination with multiple pages",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				if params.Cursor == "" {
					// First page
					return []string{"U111111", "U222222"}, "cursor_page2", nil
				} else if params.Cursor == "cursor_page2" {
					// Second page
					return []string{"U333333", "U444444"}, "", nil
				}
				return nil, "", errors.New("unexpected cursor")
			},
			mockUserInfoFunc: func(user string) (*slack.User, error) {
				return &slack.User{
					ID:      user,
					IsBot:   false,
					Deleted: false,
				}, nil
			},
			expectedMemberCount: 4,
			wantErr:             false,
		},
		{
			name: "filter out bots",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				return []string{"U111111", "UBOT123", "U222222"}, "", nil
			},
			mockUserInfoFunc: func(user string) (*slack.User, error) {
				if user == "UBOT123" {
					return &slack.User{
						ID:      user,
						IsBot:   true,
						Deleted: false,
					}, nil
				}
				return &slack.User{
					ID:      user,
					IsBot:   false,
					Deleted: false,
				}, nil
			},
			expectedMemberCount: 2,
			wantErr:             false,
		},
		{
			name: "filter out deleted users",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				return []string{"U111111", "UDEL456", "U222222"}, "", nil
			},
			mockUserInfoFunc: func(user string) (*slack.User, error) {
				if user == "UDEL456" {
					return &slack.User{
						ID:      user,
						IsBot:   false,
						Deleted: true,
					}, nil
				}
				return &slack.User{
					ID:      user,
					IsBot:   false,
					Deleted: false,
				}, nil
			},
			expectedMemberCount: 2,
			wantErr:             false,
		},
		{
			name: "filter out both bots and deleted users",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				return []string{"U111111", "UBOT123", "UDEL456", "U222222"}, "", nil
			},
			mockUserInfoFunc: func(user string) (*slack.User, error) {
				switch user {
				case "UBOT123":
					return &slack.User{ID: user, IsBot: true, Deleted: false}, nil
				case "UDEL456":
					return &slack.User{ID: user, IsBot: false, Deleted: true}, nil
				default:
					return &slack.User{ID: user, IsBot: false, Deleted: false}, nil
				}
			},
			expectedMemberCount: 2,
			wantErr:             false,
		},
		{
			name: "API error getting users",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				return nil, "", errors.New("channel_not_found")
			},
			wantErr:     true,
			errContains: "failed to get channel members",
		},
		{
			name: "continue on user info error",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				return []string{"U111111", "UBAD222", "U333333"}, "", nil
			},
			mockUserInfoFunc: func(user string) (*slack.User, error) {
				if user == "UBAD222" {
					return nil, errors.New("user_not_found")
				}
				return &slack.User{
					ID:      user,
					IsBot:   false,
					Deleted: false,
				}, nil
			},
			expectedMemberCount: 2, // UBAD222 is skipped
			wantErr:             false,
		},
		{
			name: "empty channel",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				return []string{}, "", nil
			},
			expectedMemberCount: 0,
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSlack := &ExtendedMockSlackClient{
				GetUsersInConversationFunc: tt.mockUsersFunc,
				GetUserInfoFunc:            tt.mockUserInfoFunc,
			}

			members, err := GetChannelMembers(mockSlack, "C123456")

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetChannelMembers() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("GetChannelMembers() error = %v, want error containing %s", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("GetChannelMembers() unexpected error = %v", err)
				}
				if len(members) != tt.expectedMemberCount {
					t.Errorf("GetChannelMembers() returned %d members, want %d", len(members), tt.expectedMemberCount)
				}
			}
		})
	}
}

func TestSendReminderDM(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		mockFunc    func(channelID string, options ...slack.MsgOption) (string, string, error)
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful DM send",
			userID: "U123456",
			mockFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				if channelID != "U123456" {
					t.Errorf("expected DM to user U123456, got channel %s", channelID)
				}
				return channelID, "1234567890.123456", nil
			},
			wantErr: false,
		},
		{
			name:   "Slack API error",
			userID: "U789012",
			mockFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "", "", errors.New("user_not_found")
			},
			wantErr:     true,
			errContains: "failed to send DM",
		},
		{
			name:   "rate limit error",
			userID: "U555555",
			mockFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return "", "", errors.New("rate_limited")
			},
			wantErr:     true,
			errContains: "failed to send DM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSlack := &ExtendedMockSlackClient{
				MockSlackClient: MockSlackClient{
					PostMessageFunc: tt.mockFunc,
				},
			}

			err := SendReminderDM(mockSlack, tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SendReminderDM() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("SendReminderDM() error = %v, want error containing %s", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("SendReminderDM() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestFormatReminderBlocks(t *testing.T) {
	blocks := FormatReminderBlocks()

	// Verify we have the expected number of blocks
	expectedBlockCount := 5 // Header, Section, Action, Divider, Context
	if len(blocks) != expectedBlockCount {
		t.Errorf("FormatReminderBlocks() returned %d blocks, want %d", len(blocks), expectedBlockCount)
	}

	// Verify first block is a header
	if blocks[0].BlockType() != slack.MBTHeader {
		t.Errorf("First block should be a header, got %s", blocks[0].BlockType())
	}

	// Verify second block is a section
	if blocks[1].BlockType() != slack.MBTSection {
		t.Errorf("Second block should be a section, got %s", blocks[1].BlockType())
	}

	// Verify third block is an action (button)
	if blocks[2].BlockType() != slack.MBTAction {
		t.Errorf("Third block should be an action, got %s", blocks[2].BlockType())
	}

	// Verify fourth block is a divider
	if blocks[3].BlockType() != slack.MBTDivider {
		t.Errorf("Fourth block should be a divider, got %s", blocks[3].BlockType())
	}

	// Verify fifth block is a context
	if blocks[4].BlockType() != slack.MBTContext {
		t.Errorf("Fifth block should be a context, got %s", blocks[4].BlockType())
	}
}

func TestFormatReminderBlocks_Content(t *testing.T) {
	blocks := FormatReminderBlocks()

	// Check header content
	headerBlock, ok := blocks[0].(*slack.HeaderBlock)
	if !ok {
		t.Fatal("First block is not a HeaderBlock")
	}
	if headerBlock.Text.Text != "👋 Lembrete Semanal" {
		t.Errorf("Header text = %q, want %q", headerBlock.Text.Text, "👋 Lembrete Semanal")
	}

	// Check section content
	sectionBlock, ok := blocks[1].(*slack.SectionBlock)
	if !ok {
		t.Fatal("Second block is not a SectionBlock")
	}
	expectedText := "Esta semana alguém realizou algum trabalho excepcional?\n\nUse `/elogie` para enviar um elogio e valorizar as pessoas!"
	if sectionBlock.Text.Text != expectedText {
		t.Errorf("Section text mismatch.\nGot: %q\nWant: %q", sectionBlock.Text.Text, expectedText)
	}

	// Check action block has a button
	actionBlock, ok := blocks[2].(*slack.ActionBlock)
	if !ok {
		t.Fatal("Third block is not an ActionBlock")
	}
	if len(actionBlock.Elements.ElementSet) == 0 {
		t.Error("Action block should have at least one element (button)")
	}

	// Check context block content
	contextBlock, ok := blocks[4].(*slack.ContextBlock)
	if !ok {
		t.Fatal("Fifth block is not a ContextBlock")
	}
	if len(contextBlock.ContextElements.Elements) == 0 {
		t.Error("Context block should have elements")
	}
}
