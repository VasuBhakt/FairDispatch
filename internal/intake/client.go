package intake

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// DispatchRequest matches the struct in worker, representing the incoming request
type DispatchRequest struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

type Client struct {
	sqsClient *sqs.Client
	queueURL  string
}

func NewClient(sqsClient *sqs.Client, queueURL string) *Client {
	return &Client{
		sqsClient: sqsClient,
		queueURL:  queueURL,
	}
}

// PublishRequest sends a dispatch request to the SQS queue
func (c *Client) PublishRequest(ctx context.Context, req DispatchRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = c.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("failed to send message to sqs: %w", err)
	}

	return nil
}
