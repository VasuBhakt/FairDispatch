package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	Zone           string         `dynamodbav:"zone"`
	Status         ResourceStatus `dynamodbav:"status"`
	OrdersLastHour int            `dynamodbav:"orders_last_hour"`
	IdleSince      int64          `dynamodbav:"idle_since"`
	HeldAt         int64          `dynamodbav:"held_at"`
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
		UpdateExpression: aws.String("SET #s = :held, assigned_request = :reqId, held_at = :now"),
		// Critical correctness check: It MUST still be AVAILABLE right at the moment of the write
		ConditionExpression: aws.String("#s = :available"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":held":      &types.AttributeValueMemberS{Value: string(StatusHeld)},
			":reqId":     &types.AttributeValueMemberS{Value: requestID},
			":available": &types.AttributeValueMemberS{Value: string(StatusAvailable)},
			":now":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
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

// ConfirmResource moves a resource from HELD to BUSY.
func ConfirmResource(ctx context.Context, client *dynamodb.Client, resourceID string) error {
	_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("resources"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: resourceID},
		},
		UpdateExpression:    aws.String("SET #s = :busy"),
		ConditionExpression: aws.String("#s = :held"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":busy": &types.AttributeValueMemberS{Value: string(StatusBusy)},
			":held": &types.AttributeValueMemberS{Value: string(StatusHeld)},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to confirm resource: %w", err)
	}
	return nil
}

// CompleteResource moves a resource from BUSY to AVAILABLE, updating stats.
func CompleteResource(ctx context.Context, client *dynamodb.Client, resourceID string) error {
	_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("resources"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: resourceID},
		},
		UpdateExpression:    aws.String("SET #s = :available, orders_last_hour = orders_last_hour + :inc, idle_since = :now"),
		ConditionExpression: aws.String("#s = :busy"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":available": &types.AttributeValueMemberS{Value: string(StatusAvailable)},
			":busy":      &types.AttributeValueMemberS{Value: string(StatusBusy)},
			":inc":       &types.AttributeValueMemberN{Value: "1"},
			":now":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to complete resource: %w", err)
	}
	return nil
}

// ReleaseSpecificHold releases a specific HELD resource back to AVAILABLE if it is still
// HELD by the same requestID. This is an O(1) conditional update.
func ReleaseSpecificHold(ctx context.Context, client *dynamodb.Client, resourceID string, requestID string) error {
	_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("resources"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: resourceID},
		},
		UpdateExpression:    aws.String("SET #s = :available, idle_since = :now REMOVE assigned_request, held_at"),
		ConditionExpression: aws.String("#s = :held AND assigned_request = :reqId"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":available": &types.AttributeValueMemberS{Value: string(StatusAvailable)},
			":held":      &types.AttributeValueMemberS{Value: string(StatusHeld)},
			":now":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())},
			":reqId":     &types.AttributeValueMemberS{Value: requestID},
		},
	})
	if err != nil {
		// If the condition check fails, it means the driver already confirmed/completed,
		// or someone else got it. We just ignore the error.
		var condCheckFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condCheckFailed) || strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			return nil
		}
		return fmt.Errorf("failed to release specific hold: %w", err)
	}
	return nil
}
