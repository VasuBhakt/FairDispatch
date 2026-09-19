package api

import (
	"encoding/json"
	"net/http"

	"fair-dispatch/internal/domain"
	"fair-dispatch/internal/intake"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Handler struct {
	intakeClient *intake.Client
	dbClient     *dynamodb.Client
}

func NewHandler(intakeClient *intake.Client, dbClient *dynamodb.Client) *Handler {
	return &Handler{
		intakeClient: intakeClient,
		dbClient:     dbClient,
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

type resourceRequest struct {
	ResourceID string `json:"resource_id"`
}

func (h *Handler) ConfirmHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req resourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := domain.ConfirmResource(r.Context(), h.dbClient, req.ResourceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) CompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req resourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := domain.CompleteResource(r.Context(), h.dbClient, req.ResourceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	// A naive scan just for the dashboard metrics
	out, err := h.dbClient.Scan(r.Context(), &dynamodb.ScanInput{
		TableName: aws.String("resources"),
	})
	if err != nil {
		http.Error(w, "Failed to scan", http.StatusInternalServerError)
		return
	}

	metrics := map[string]int{
		"AVAILABLE": 0,
		"HELD":      0,
		"BUSY":      0,
	}

	for _, item := range out.Items {
		if statusAttr, ok := item["status"]; ok {
			if s, ok := statusAttr.(*types.AttributeValueMemberS); ok {
				metrics[s.Value]++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(metrics)
}
