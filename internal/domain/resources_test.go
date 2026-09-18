package domain_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"fair-dispatch/internal/db"
	"fair-dispatch/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestAtomicClaimRaceCondition(t *testing.T) {
	ctx := context.Background()
	clients := db.NewLocalClients(ctx)

	// 1. Setup: Create a fresh resource in DynamoDB
	resourceID := fmt.Sprintf("rider-test-%d", time.Now().UnixNano())

	_, err := clients.DynamoDB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("resources"),
		Item: map[string]types.AttributeValue{
			"id":     &types.AttributeValueMemberS{Value: resourceID},
			"status": &types.AttributeValueMemberS{Value: string(domain.StatusAvailable)},
			"type":   &types.AttributeValueMemberS{Value: "food-rider"},
		},
	})
	if err != nil {
		t.Fatalf("failed to setup test resource: %v", err)
	}

	// 2. The Race: Two requests arrive at the exact same time trying to claim it
	var wg sync.WaitGroup
	var successCount int
	var failureCount int
	var mu sync.Mutex

	runClaim := func(requestID string) {
		defer wg.Done()

		err := domain.ClaimResource(ctx, clients.DynamoDB, resourceID, requestID)

		mu.Lock()
		defer mu.Unlock()
		if err == nil {
			successCount++
		} else if errors.Is(err, domain.ErrAlreadyClaimed) {
			failureCount++
		} else {
			t.Errorf("unexpected error from claim: %v", err)
		}
	}

	wg.Add(2)
	// We use goroutines to trigger them concurrently!
	go runClaim("request-A")
	go runClaim("request-B")

	wg.Wait()

	// 3. Assertions: Exactly one winner, exactly one loser
	if successCount != 1 {
		t.Errorf("expected exactly 1 successful claim, got %d", successCount)
	}
	if failureCount != 1 {
		t.Errorf("expected exactly 1 failed claim, got %d", failureCount)
	}
}
