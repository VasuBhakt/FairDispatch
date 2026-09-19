package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"fair-dispatch/internal/api"
	"fair-dispatch/internal/db"
	"fair-dispatch/internal/intake"
)

func main() {
	ctx := context.Background()

	// Initialize AWS clients pointing to LocalStack
	clients := db.NewLocalClients(ctx)

	// In a real app, we'd look this up using GetQueueUrl
	queueURL := "http://sqs.us-east-1.localhost.localstack.cloud:4566/000000000000/request-intake"
	if os.Getenv("QUEUE_URL") != "" {
		queueURL = os.Getenv("QUEUE_URL")
	}

	intakeClient := intake.NewClient(clients.SQS, queueURL)
	handler := api.NewHandler(intakeClient)

	http.HandleFunc("/dispatch", handler.DispatchHandler)

	port := ":8080"
	log.Printf("Starting API server on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
