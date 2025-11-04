package reminder

import (
	"context"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/slack-go/slack"
	"github.com/vyper/my-matter/internal/config"
)

// MockSlackClient is a mock implementation of config.SlackClient for reminder tests
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
	return channelID, "1234567890.123456", nil
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

func setupTestConfig(t *testing.T, mockSlack *MockSlackClient) {
	globalConfig = &config.Config{
		SlackBotToken:  "xoxb-test-token",
		SlackChannelID: "C123456",
		SigningSecret:  "test-secret",
		SlackAPI:       mockSlack,
	}
}

func TestHandleReminder(t *testing.T) {
	tests := []struct {
		name               string
		mockUsersFunc      func(params *slack.GetUsersInConversationParameters) ([]string, string, error)
		mockUserInfoFunc   func(user string) (*slack.User, error)
		mockPostMsgFunc    func(channelID string, options ...slack.MsgOption) (string, string, error)
		expectedDMCount    int
		expectError        bool
	}{
		{
			name: "successful reminder to multiple users",
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
			mockPostMsgFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return channelID, "1234567890.123456", nil
			},
			expectedDMCount: 3,
			expectError:     false,
		},
		{
			name: "filter out bots and send to real users",
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
			mockPostMsgFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				return channelID, "1234567890.123456", nil
			},
			expectedDMCount: 2,
			expectError:     false,
		},
		{
			name: "continue sending even if some DMs fail",
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
			mockPostMsgFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
				// Fail for U222222, succeed for others
				if channelID == "U222222" {
					return "", "", &slack.SlackErrorResponse{Err: "user_not_found"}
				}
				return channelID, "1234567890.123456", nil
			},
			expectedDMCount: 3,
			expectError:     false, // Function returns nil even with partial failures
		},
		{
			name: "no users in channel",
			mockUsersFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
				return []string{}, "", nil
			},
			mockUserInfoFunc: func(user string) (*slack.User, error) {
				return &slack.User{
					ID:      user,
					IsBot:   false,
					Deleted: false,
				}, nil
			},
			expectedDMCount: 0,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dmCount := 0
			mockSlack := &MockSlackClient{
				GetUsersInConversationFunc: tt.mockUsersFunc,
				GetUserInfoFunc:            tt.mockUserInfoFunc,
				PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
					dmCount++
					if tt.mockPostMsgFunc != nil {
						return tt.mockPostMsgFunc(channelID, options...)
					}
					return channelID, "1234567890.123456", nil
				},
			}

			setupTestConfig(t, mockSlack)

			// Create a CloudEvent
			e := event.New()
			e.SetID("test-event-id")
			e.SetSource("test-source")
			e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
			e.SetTime(time.Now())
			e.SetData("application/json", map[string]interface{}{
				"trigger": "weekly_reminder",
			})

			err := handleReminder(context.Background(), e)

			if tt.expectError && err == nil {
				t.Error("handleReminder() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("handleReminder() unexpected error = %v", err)
			}

			if dmCount != tt.expectedDMCount {
				t.Errorf("handleReminder() sent %d DMs, want %d", dmCount, tt.expectedDMCount)
			}
		})
	}
}

func TestHandleReminder_GetChannelMembersError(t *testing.T) {
	mockSlack := &MockSlackClient{
		GetUsersInConversationFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
			return nil, "", &slack.SlackErrorResponse{Err: "channel_not_found"}
		},
		PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			t.Error("PostMessage should not be called when GetUsersInConversation fails")
			return "", "", nil
		},
	}

	setupTestConfig(t, mockSlack)

	e := event.New()
	e.SetID("test-event-id")
	e.SetSource("test-source")
	e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
	e.SetTime(time.Now())

	err := handleReminder(context.Background(), e)

	if err == nil {
		t.Error("handleReminder() expected error when GetChannelMembers fails, got nil")
	}
}

func TestHandleReminder_Pagination(t *testing.T) {
	dmCount := 0
	mockSlack := &MockSlackClient{
		GetUsersInConversationFunc: func(params *slack.GetUsersInConversationParameters) ([]string, string, error) {
			if params.Cursor == "" {
				// First page
				return []string{"U111111", "U222222"}, "cursor_page2", nil
			} else if params.Cursor == "cursor_page2" {
				// Second page
				return []string{"U333333"}, "", nil
			}
			return nil, "", &slack.SlackErrorResponse{Err: "invalid_cursor"}
		},
		GetUserInfoFunc: func(user string) (*slack.User, error) {
			return &slack.User{
				ID:      user,
				IsBot:   false,
				Deleted: false,
			}, nil
		},
		PostMessageFunc: func(channelID string, options ...slack.MsgOption) (string, string, error) {
			dmCount++
			return channelID, "1234567890.123456", nil
		},
	}

	setupTestConfig(t, mockSlack)

	e := event.New()
	e.SetID("test-event-id")
	e.SetSource("test-source")
	e.SetType("google.cloud.pubsub.topic.v1.messagePublished")
	e.SetTime(time.Now())

	err := handleReminder(context.Background(), e)

	if err != nil {
		t.Errorf("handleReminder() unexpected error = %v", err)
	}

	expectedDMs := 3
	if dmCount != expectedDMs {
		t.Errorf("handleReminder() sent %d DMs, want %d (across paginated results)", dmCount, expectedDMs)
	}
}
