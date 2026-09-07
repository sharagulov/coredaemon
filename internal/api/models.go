package api

import (
	"net/http"

	"github.com/core-daemon/core-daemon/internal/ai"
)

// MountModels registers GET /api/models for the chat provider picker.
func MountModels(mux *http.ServeMux, agent *ai.Agent) {
	mux.HandleFunc("GET /api/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Providers []ai.ModelProvider `json:"providers"`
		}{Providers: agent.Providers()})
	})
}
