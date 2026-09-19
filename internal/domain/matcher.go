package domain

import (
	"context"
	"fair-dispatch/internal/config"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Matcher calculates the best resource for a request.
type Matcher struct {
	Config *config.DomainConfig
}

func NewMatcher(cfg *config.DomainConfig) *Matcher {
	return &Matcher{Config: cfg}
}

// Score evaluates a candidate resource. Higher score is better.
func (m *Matcher) Score(resource *Resource, distanceToPickup float64) float64 {
	// 1. Fairness Score: Inverse of orders completed. 0 orders = 1.0 (Max)
	fairness := 1.0 / (float64(resource.OrdersLastHour) + 1.0)
	// 2. Proximity Score: Inverse of distance. Distance 0 = 1.0 (Max)
	proximity := 1.0 / (distanceToPickup + 1.0)
	// Weighted sum based on YAML config
	return (fairness * m.Config.Weights.Fairness) + (proximity * m.Config.Weights.Proximity)
}

// FindBestResource queries the GSI for AVAILABLE resources in the given zone and returns the highest scoring one.
func (m *Matcher) FindBestResource(ctx context.Context, client *dynamodb.Client, zone string) (*Resource, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String("resources"),
		IndexName:              aws.String("ZoneStatusIndex"),
		KeyConditionExpression: aws.String("#z = :zone AND #s = :available"),
		FilterExpression:       aws.String("#t = :rtype"),
		ExpressionAttributeNames: map[string]string{
			"#z": "zone",
			"#s": "status",
			"#t": "type",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zone":      &types.AttributeValueMemberS{Value: zone},
			":available": &types.AttributeValueMemberS{Value: string(StatusAvailable)},
			":rtype":     &types.AttributeValueMemberS{Value: m.Config.ResourceType},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan resources: %w", err)
	}
	var candidates []Resource
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &candidates); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available resources found")
	}
	var bestResource *Resource
	var highestScore float64 = -1.0
	for i := range candidates {
		candidate := &candidates[i]

		// Mock distance
		mockDistance := 2.0
		score := m.Score(candidate, mockDistance)
		log.Printf("Evaluated resource %s: score=%.2f (orders=%d)", candidate.ID, score, candidate.OrdersLastHour)
		if score > highestScore {
			highestScore = score
			bestResource = candidate
		}
	}
	return bestResource, nil
}
