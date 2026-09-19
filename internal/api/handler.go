package api

import (
	"encoding/json"
	"net/http"

	"fair-dispatch/internal/intake"
)

type Handler struct {
	intakeClient *intake.Client
}

func NewHandler(intakeClient *intake.Client) *Handler {
	return &Handler{
		intakeClient: intakeClient,
	}
}

func (h *Handler) DispatchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req intake.DispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.Domain == "" {
		http.Error(w, "Missing id or domain", http.StatusBadRequest)
		return
	}

	err := h.intakeClient.PublishRequest(r.Context(), req)
	if err != nil {
		http.Error(w, "Failed to publish request", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
		"id":     req.ID,
	})
}
