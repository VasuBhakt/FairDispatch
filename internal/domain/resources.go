package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type ResourceStatus string

const (
	StatusAvailable ResourceStatus = "AVAILABLE"
	StatusHeld      ResourceStatus = "HELD"
	StatusBusy      ResourceStatus = "BUSY"
)

type Resource struct {
	ID             string         `dynamodbav:"id"`
	Type           string         `dynamodbav:"type"`
	Status         ResourceStatus `dynamodbav:"status"`
	OrdersLastHour int            `dynamodbav:"orders_last_hour"`
	IdleSince      int64          `dynamodbav:"idle_since"`
}

// ErrAlreadyClaimed is returned when the atomic claim fails due to a race condition.
var ErrAlreadyClaimed = fmt.Errorf("resource is no longer available")

// ClaimResource atomically claims a resource for a specific request.
// It uses DynamoDB conditional expressions to ensure status = AVAILABLE.
func ClaimResource(ctx context.Context, client *dynamodb.Client, resourceID string, requestID string) error {
	_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("resources"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: resourceID},
		},
		// Update the status and associate it with the request
		UpdateExpression: aws.String("SET #s = :held, assigned_request = :reqId"),
		// Critical correctness check: It MUST still be AVAILABLE right at the moment of the write
		ConditionExpression: aws.String("#s = :available"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":held":      &types.AttributeValueMemberS{Value: string(StatusHeld)},
			":reqId":     &types.AttributeValueMemberS{Value: requestID},
			":available": &types.AttributeValueMemberS{Value: string(StatusAvailable)},
		},
	})
	if err != nil {
		// Check if it's a ConditionalCheckFailedException (meaning we lost the race)
		var condCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckFailed) || strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			return ErrAlreadyClaimed
		}
		return fmt.Errorf("failed to claim resource: %w", err)
	}
	return nil
}

// SaveResource saves a resource to the DynamoDB resources table.
func SaveResource(ctx context.Context, client *dynamodb.Client, resource Resource) error {
	item, err := attributevalue.MarshalMap(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("resources"),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to put item in dynamodb: %w", err)
	}
	return nil
}
