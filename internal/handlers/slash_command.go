package handlers

import (
	"log"
	"net/http"

	"github.com/vyper/my-matter/internal/config"
	"github.com/vyper/my-matter/internal/services"
)

// HandleSlashCommand processes the /elogie slash command and opens the kudos modal
func HandleSlashCommand(w http.ResponseWriter, r *http.Request, viewTemplate string, cfg *config.Config) {
	triggerID := r.FormValue("trigger_id")
	if triggerID == "" {
		log.Printf("Missing trigger_id in slash command")
		http.Error(w, "Missing trigger_id", http.StatusBadRequest)
		return
	}

	err := services.OpenModal(triggerID, viewTemplate, cfg)
	if err != nil {
		log.Printf("Error opening modal: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
