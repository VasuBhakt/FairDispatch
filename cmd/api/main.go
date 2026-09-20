package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"fair-dispatch/internal/api"
	"fair-dispatch/internal/db"
	"fair-dispatch/internal/intake"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	ctx := context.Background()

	// Initialize AWS clients pointing to LocalStack
	clients := db.NewLocalClients(ctx)

	queueURL := os.Getenv("QUEUE_URL")
	if queueURL == "" {
		log.Fatal("QUEUE_URL is not set")
	}

	intakeClient := intake.NewClient(clients.SQS, queueURL)
	handler := api.NewHandler(intakeClient, clients.DynamoDB)

	http.HandleFunc("/dispatch", handler.DispatchHandler)
	http.HandleFunc("/confirm", handler.ConfirmHandler)
	http.HandleFunc("/complete", handler.CompleteHandler)
	http.HandleFunc("/metrics", handler.MetricsHandler)
	http.Handle("/", http.FileServer(http.Dir("web")))

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}
	log.Printf("Starting API server on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
