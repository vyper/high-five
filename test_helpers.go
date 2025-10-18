package function

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

// MockSlackClient is a mock implementation of the Slack client for testing
type MockSlackClient struct {
	PostMessageFunc func(channelID string, options ...slack.MsgOption) (string, string, error)
}

func (m *MockSlackClient) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	if m.PostMessageFunc != nil {
		return m.PostMessageFunc(channelID, options...)
	}
	return channelID, "1234567890.123456", nil
}

// GenerateSlackSignature generates a valid Slack signature for testing
func GenerateSlackSignature(secret, body string, timestamp int64) string {
	sig := fmt.Sprintf("v0:%d:%s", timestamp, body)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(sig))
	return "v0=" + hex.EncodeToString(h.Sum(nil))
}

// CreateSlackRequest creates an HTTP request with valid Slack signature headers
func CreateSlackRequest(method, contentType, body, signingSecret string) *http.Request {
	timestamp := time.Now().Unix()
	signature := GenerateSlackSignature(signingSecret, body, timestamp)

	req := &http.Request{
		Method: method,
		Header: http.Header{
			"Content-Type":              {contentType},
			"X-Slack-Request-Timestamp": {strconv.FormatInt(timestamp, 10)},
			"X-Slack-Signature":         {signature},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}

	return req
}

// ValidInteractionCallbackPayload returns a valid JSON payload for modal submission
func ValidInteractionCallbackPayload() string {
	return `{
		"type": "view_submission",
		"user": {
			"id": "U12345678",
			"username": "testuser"
		},
		"view": {
			"state": {
				"values": {
					"kudo_users": {
						"kudo_users": {
							"type": "multi_users_select",
							"selected_users": ["U87654321", "U11111111"]
						}
					},
					"kudo_type": {
						"kudo_type": {
							"type": "static_select",
							"selected_option": {
								"text": {
									"type": "plain_text",
									"text": ":dart: Entrega Excepcional"
								},
								"value": "value-0"
							}
						}
					},
					"kudo_message": {
						"kudo_message": {
							"type": "plain_text_input",
							"value": "Great work on the project!"
						}
					}
				}
			}
		}
	}`
}

// ValidInteractionCallbackPayloadNoMessage returns a payload without optional message
func ValidInteractionCallbackPayloadNoMessage() string {
	return `{
		"type": "view_submission",
		"user": {
			"id": "U12345678",
			"username": "testuser"
		},
		"view": {
			"state": {
				"values": {
					"kudo_users": {
						"kudo_users": {
							"type": "multi_users_select",
							"selected_users": ["U87654321"]
						}
					},
					"kudo_type": {
						"kudo_type": {
							"type": "static_select",
							"selected_option": {
								"text": {
									"type": "plain_text",
									"text": ":handshake: Espírito de Equipe"
								},
								"value": "value-1"
							}
						}
					},
					"kudo_message": {
						"kudo_message": {
							"type": "plain_text_input",
							"value": ""
						}
					}
				}
			}
		}
	}`
}

// InvalidJSONPayload returns malformed JSON
func InvalidJSONPayload() string {
	return `{"type": "view_submission", "user": {invalid json`
}
