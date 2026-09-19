package main

import (
	"context"
	"log"
	"os"

	"fair-dispatch/internal/config"
	"fair-dispatch/internal/db"
	"fair-dispatch/internal/worker"

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
