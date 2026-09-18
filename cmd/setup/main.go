package main

import (
	"context"
	"log"

	"fair-dispatch/internal/db"
)

func main() {
	ctx := context.Background()

	// Initialize AWS clients pointing to LocalStack
	clients := db.NewLocalClients(ctx)

	// Run the setup script to create tables and queues
	err := db.Setup(ctx, clients)
	if err != nil {
		log.Fatalf("Failed to setup database resources: %v", err)
	}
}
