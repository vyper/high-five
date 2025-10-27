package function

import (
	"github.com/vyper/my-matter/functions/interactivity"
	"net/http"
)

// Interactivity is the Cloud Function entry point
func Interactivity(w http.ResponseWriter, r *http.Request) {
	interactivity.HandleInteractivity(w, r)
}
