package db

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type Clients struct {
	DynamoDB *dynamodb.Client
	SQS      *sqs.Client
}

// NewLocalClients initializes AWS SDK clients pointing to LocalStack
func NewLocalClients(ctx context.Context) *Clients {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	localEndpoint := aws.String("http://localhost:4566")

	return &Clients{
		DynamoDB: dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.BaseEndpoint = localEndpoint
		}),
		SQS: sqs.NewFromConfig(cfg, func(o *sqs.Options) {
			o.BaseEndpoint = localEndpoint
		}),
	}
}
