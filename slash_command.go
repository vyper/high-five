package function

import (
	"github.com/vyper/my-matter/functions/slashcommand"
	"net/http"
)

// SlashCommand is the Cloud Function entry point
func SlashCommand(w http.ResponseWriter, r *http.Request) {
	slashcommand.HandleSlashCommand(w, r)
}
