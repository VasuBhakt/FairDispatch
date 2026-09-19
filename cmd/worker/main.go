package main

import (
	"context"
	"log"
	"os"

	"fair-dispatch/internal/config"
	"fair-dispatch/internal/db"
	"fair-dispatch/internal/worker"
)

func main() {
	ctx := context.Background()

	// Initialize AWS clients pointing to LocalStack
	clients := db.NewLocalClients(ctx)

	queueURL := "http://sqs.us-east-1.localhost.localstack.cloud:4566/000000000000/request-intake"
	if os.Getenv("QUEUE_URL") != "" {
		queueURL = os.Getenv("QUEUE_URL")
	}

	dispatcher := worker.NewDispatcher(clients.SQS, clients.DynamoDB, queueURL)

	// Load domains
	foodCfg, err := config.LoadConfig("configs/food_delivery.yaml")
	if err != nil {
		log.Fatalf("Failed to load food_delivery config: %v", err)
	}
	dispatcher.RegisterDomain(foodCfg)

	// Load cabs config if it exists
	cabsCfg, err := config.LoadConfig("configs/cabs.yaml")
	if err != nil {
		log.Fatalf("Failed to load cabs config: %v", err)
	}
	dispatcher.RegisterDomain(cabsCfg)

	dispatcher.Start(ctx)
}
