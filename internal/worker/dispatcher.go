package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fair-dispatch/internal/config"
	"fair-dispatch/internal/domain"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type DispatchRequest struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

type Dispatcher struct {
	sqsClient *sqs.Client
	dbClient  *dynamodb.Client
	queueURL  string
	configs   map[string]*config.DomainConfig
}

func NewDispatcher(sqsClient *sqs.Client, dbClient *dynamodb.Client, queueURL string) *Dispatcher {
	return &Dispatcher{
		sqsClient: sqsClient,
		dbClient:  dbClient,
		queueURL:  queueURL,
		configs:   make(map[string]*config.DomainConfig),
	}
}

// RegisterDomain tells the dispatcher which config to use for which domain requests
func (d *Dispatcher) RegisterDomain(cfg *config.DomainConfig) {
	d.configs[cfg.Domain] = cfg
}

func (d *Dispatcher) poll(ctx context.Context) {
	// Long-polling SQS (WaitTimeSeconds: 10) to save CPU and API calls
	msgResult, err := d.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(d.queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     10,
	})
	if err != nil {
		log.Printf("Error receiving message: %v", err)
		time.Sleep(2 * time.Second)
		return
	}
	if len(msgResult.Messages) == 0 {
		return // No messages in queue
	}
	msg := msgResult.Messages[0]
	var req DispatchRequest
	if err := json.Unmarshal([]byte(*msg.Body), &req); err != nil {
		log.Printf("Failed to parse request JSON: %v", err)
		d.deleteMessage(ctx, msg.ReceiptHandle) // Drop malformed messages
		return
	}
	log.Printf("---")
	log.Printf("Processing request %s for domain '%s'", req.ID, req.Domain)
	cfg, ok := d.configs[req.Domain]
	if !ok {
		log.Printf("Unknown domain config: %s", req.Domain)
		return
	}
	matcher := domain.NewMatcher(cfg)

	// Retry loop for our atomic claim race condition
	for i := 0; i < 3; i++ {
		bestResource, err := matcher.FindBestResource(ctx, d.dbClient)
		if err != nil {
			log.Printf("No available resources found: %v (Wait for next cycle)", err)
			time.Sleep(2 * time.Second)
			return // The message will reappear in SQS after visibility timeout
		}
		err = domain.ClaimResource(ctx, d.dbClient, bestResource.ID, req.ID)
		if err == nil {
			log.Printf("✅ SUCCESS: Atomically claimed resource '%s' for request '%s'", bestResource.ID, req.ID)
			d.deleteMessage(ctx, msg.ReceiptHandle) // We are done with this message!
			return
		}
		if errors.Is(err, domain.ErrAlreadyClaimed) {
			log.Printf("⚠️ Race lost for resource %s! Someone else claimed it. Retrying immediately...", bestResource.ID)
			continue // Try to find the NEXT best resource immediately
		}
		log.Printf("Failed to claim resource: %v", err)
		break
	}
}

func (d *Dispatcher) deleteMessage(ctx context.Context, receiptHandle *string) {
	_, err := d.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(d.queueURL),
		ReceiptHandle: receiptHandle,
	})
	if err != nil {
		log.Printf("Failed to delete message from queue: %v", err)
	}
}

// Start begins a persistent long-polling loop against the SQS queue
func (d *Dispatcher) Start(ctx context.Context) {
	log.Println("Dispatcher worker starting...")
	for {
		select {
		case <-ctx.Done():
			log.Println("Dispatcher worker stopping...")
			return
		default:
			d.poll(ctx)
		}
	}
}
