package function

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/vyper/my-matter/functions/reminder"
)

// Reminder is the Cloud Function entry point for Pub/Sub triggered reminders
func Reminder(ctx context.Context, e event.Event) error {
	return reminder.HandleReminder(ctx, e)
}
