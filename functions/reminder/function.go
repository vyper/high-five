package reminder

import (
	"context"
	"log"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/services"
)

var globalConfig *config.Config

func init() {
	functions.CloudEvent("Reminder", handleReminder)

	cfg, err := config.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	globalConfig = cfg
}

// PubSubMessage represents a Pub/Sub message
type PubSubMessage struct {
	Data []byte `json:"data"`
}

func handleReminder(ctx context.Context, e event.Event) error {
	log.Printf("Reminder function triggered at %s", e.Time())

	// Get channel members
	members, err := services.GetChannelMembers(globalConfig.SlackAPI, globalConfig.SlackChannelID)
	if err != nil {
		log.Printf("Error getting channel members: %v", err)
		return err
	}

	log.Printf("Found %d active members to send reminders to", len(members))

	// Send DM to each member
	successCount := 0
	errorCount := 0

	for _, userID := range members {
		err := services.SendReminderDM(globalConfig.SlackAPI, userID)
		if err != nil {
			log.Printf("Failed to send reminder to user %s: %v", userID, err)
			errorCount++
		} else {
			log.Printf("Successfully sent reminder to user %s", userID)
			successCount++
		}
	}

	log.Printf("Reminder sending complete. Success: %d, Errors: %d", successCount, errorCount)

	// Return nil even if some DMs failed - we don't want to retry for partial failures
	return nil
}

// HandleReminder is the exported function for the Cloud Function entry point
func HandleReminder(ctx context.Context, e event.Event) error {
	return handleReminder(ctx, e)
}
