package main

import (
	"context"
	"log"

	"fair-dispatch/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	ctx := context.Background()

	// Initialize AWS clients pointing to LocalStack
	clients := db.NewLocalClients(ctx)

	// Run the setup script to create tables and queues
	err := db.Setup(ctx, clients)
	if err != nil {
		log.Fatalf("Failed to setup database resources: %v", err)
	}
}
