package db

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Setup creates the necessary DynamoDB tables and SQS queue for the application
func Setup(ctx context.Context, clients *Clients) error {
	log.Println("Setting up local database resources...")
	if err := CreateResourcesTable(ctx, clients.DynamoDB); err != nil {
		return err
	}
	if err := CreateRequestsTable(ctx, clients.DynamoDB); err != nil {
		return err
	}
	if err := CreateIntakeQueue(ctx, clients.SQS); err != nil {
		return err
	}
	log.Println("Database setup complete.")
	return nil
}

func CreateResourcesTable(ctx context.Context, client *dynamodb.Client) error {
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String("resources"),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		var resourceInUseException *types.ResourceInUseException
		if errors.As(err, &resourceInUseException) || strings.Contains(err.Error(), "ResourceInUseException") {
			log.Println("Table 'resources' already exists.")
			return nil
		}
		return err
	}
	log.Println("Created table 'resources'.")
	return nil
}

func CreateRequestsTable(ctx context.Context, client *dynamodb.Client) error {
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String("requests"),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		var resourceInUseException *types.ResourceInUseException
		if errors.As(err, &resourceInUseException) || strings.Contains(err.Error(), "ResourceInUseException") {
			log.Println("Table 'requests' already exists.")
			return nil
		}
		return err
	}
	log.Println("Created table 'requests'.")
	return nil
}

func CreateIntakeQueue(ctx context.Context, client *sqs.Client) error {
	_, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String("request-intake"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "QueueNameExists") {
			log.Println("Queue 'request-intake' already exists.")
			return nil
		}
		return err
	}
	log.Println("Created queue 'request-intake'.")
	return nil
}
