package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"fair-dispatch/internal/db"
	"fair-dispatch/internal/domain"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func main() {
	ctx := context.Background()
	clients := db.NewLocalClients(ctx)

	log.Println("Setting up load test...")

	// 1. Seed 5 available cab drivers
	seedResources(ctx, clients.DynamoDB)

	// 2. Fire 50 concurrent requests
	log.Println("Firing 50 concurrent requests to API...")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(reqID int) {
			defer wg.Done()
			sendRequest(fmt.Sprintf("load-req-%d", reqID))
		}(i)
	}

	wg.Wait()
	log.Println("All requests sent. Waiting for workers to process...")
	time.Sleep(5 * time.Second) // Give workers some time to poll and process
	log.Println("Load test complete. Check worker logs and dynamo DB state.")
}

func seedResources(ctx context.Context, client *dynamodb.Client) {
	for i := 1; i <= 5; i++ {
		resource := domain.Resource{
			ID:             fmt.Sprintf("cab-%d", i),
			Type:           "cab-driver",
			Status:         domain.StatusAvailable,
			OrdersLastHour: 0,
			IdleSince:      time.Now().Unix(),
		}
		err := domain.SaveResource(ctx, client, resource)
		if err != nil {
			log.Fatalf("Failed to seed resource: %v", err)
		}
	}
	log.Println("Seeded 5 cab drivers.")
}

func sendRequest(id string) {
	payload := map[string]string{
		"id":     id,
		"domain": "cabs",
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post("http://localhost:8080/dispatch", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Request %s failed to send: %v", id, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		log.Printf("Request %s got unexpected status: %s", id, resp.Status)
	}
}
