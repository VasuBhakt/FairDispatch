package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"fair-dispatch/internal/db"
	"fair-dispatch/internal/domain"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	ctx := context.Background()
	clients := db.NewLocalClients(ctx)

	log.Println("Setting up load test...")

	// 1. Seed 15 available cab drivers across Kolkata zones
	seedResources(ctx, clients.DynamoDB)

	// 2. Fire 50 concurrent requests
	log.Println("\n=== INITIAL STATE ===")
	printMetrics()

	log.Println("\nFiring 50 concurrent requests to API...")
	var wg sync.WaitGroup
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sendRequest(fmt.Sprintf("load-req-%d", id))
		}(i)
	}

	wg.Wait()
	log.Println("\nAll requests sent. Waiting for workers to process...")
	time.Sleep(8 * time.Second)

	log.Println("\n=== STATE AFTER CLAIMS (Workers processed queue) ===")
	printMetrics()

	log.Println("\nSimulating restaurant confirmations (moving to BUSY)...")
	for i := 1; i <= 15; i++ {
		hitAPI("/confirm", fmt.Sprintf("cab-%d", i))
	}
	time.Sleep(2 * time.Second)

	log.Println("\n=== STATE AFTER CONFIRMATIONS (Drivers actively delivering) ===")
	printMetrics()

	log.Println("\nSimulating completed deliveries (moving to AVAILABLE)...")
	for i := 1; i <= 15; i++ {
		hitAPI("/complete", fmt.Sprintf("cab-%d", i))
	}
	time.Sleep(2 * time.Second)

	log.Println("\n=== FINAL STATE (Drivers returned to pool) ===")
	printMetrics()

	log.Println("\nLoad test complete. Notice how the fleet transitioned beautifully through the state machine!")
}

func hitAPI(endpoint string, resourceID string) {
	apiUrl := os.Getenv("API_URL")
	if apiUrl == "" {
		apiUrl = "http://localhost:8080"
	}
	payload := map[string]string{"resource_id": resourceID}
	body, _ := json.Marshal(payload)
	_, err := http.Post(apiUrl+endpoint, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Failed %s for %s: %v", endpoint, resourceID, err)
	}
}

func printMetrics() {
	apiUrl := os.Getenv("API_URL")
	if apiUrl == "" {
		apiUrl = "http://localhost:8080"
	}
	resp, err := http.Get(apiUrl + "/metrics")
	if err != nil {
		log.Printf("Failed to fetch metrics: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var metrics map[string]int
	json.Unmarshal(body, &metrics)

	fmt.Printf("--------------------------------------------------\n")
	fmt.Printf("  FLEET STATUS:  %d Available | %d Held | %d Busy \n", metrics["AVAILABLE"], metrics["HELD"], metrics["BUSY"])
	fmt.Printf("--------------------------------------------------\n")
}

func seedResources(ctx context.Context, client *dynamodb.Client) {
	zones := []string{"Salt Lake", "New Town", "Bangur Avenue", "Lake Town"}
	for i := 1; i <= 15; i++ {
		zone := zones[(i-1)%4]
		resource := domain.Resource{
			ID:             fmt.Sprintf("cab-%d", i),
			Type:           "cab-driver",
			Zone:           zone,
			Status:         domain.StatusAvailable,
			OrdersLastHour: 0,
			IdleSince:      time.Now().Unix(),
		}
		err := domain.SaveResource(ctx, client, resource)
		if err != nil {
			log.Fatalf("Failed to seed resource: %v", err)
		}
	}
	log.Printf("Seeded 15 cab drivers across zones: %v", zones)
}

func sendRequest(id string) {
	zones := []string{"Salt Lake", "New Town", "Bangur Avenue", "Lake Town"}
	zone := zones[rand.Intn(len(zones))]

	payload := map[string]string{
		"id":     id,
		"domain": "cabs",
		"zone":   zone,
	}
	body, _ := json.Marshal(payload)
	apiUrl := os.Getenv("API_URL")
	if apiUrl == "" {
		apiUrl = "http://localhost:8080"
	}
	resp, err := http.Post(apiUrl+"/dispatch", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Request %s failed to send: %v", id, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		log.Printf("Request %s got unexpected status: %s", id, resp.Status)
	}
}
