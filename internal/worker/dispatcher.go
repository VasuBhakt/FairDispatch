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
	Zone   string `json:"zone"`
}

type Dispatcher struct {
	sqsClient   *sqs.Client
	dbClient    *dynamodb.Client
	queueURL    string
	ttlQueueURL string
	configs     map[string]*config.DomainConfig
}

func NewDispatcher(sqsClient *sqs.Client, dbClient *dynamodb.Client, queueURL string, ttlQueueURL string) *Dispatcher {
	return &Dispatcher{
		sqsClient:   sqsClient,
		dbClient:    dbClient,
		queueURL:    queueURL,
		ttlQueueURL: ttlQueueURL,
		configs:     make(map[string]*config.DomainConfig),
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
		d.deleteMessage(ctx, d.queueURL, msg.ReceiptHandle) // Drop malformed messages
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
		bestResource, err := matcher.FindBestResource(ctx, d.dbClient, req.Zone)
		if err != nil {
			log.Printf("No available resources found: %v (Wait for next cycle)", err)
			time.Sleep(2 * time.Second)
			return // The message will reappear in SQS after visibility timeout
		}
		err = domain.ClaimResource(ctx, d.dbClient, bestResource.ID, req.ID)
		if err == nil {
			log.Printf("✅ SUCCESS: Atomically claimed resource '%s' for request '%s'", bestResource.ID, req.ID)
			d.deleteMessage(ctx, d.queueURL, msg.ReceiptHandle) // We are done with this message!

			// Schedule TTL Release (Event-Driven)
			d.scheduleTTLRelease(ctx, bestResource.ID, req.ID, cfg.HoldTTLSeconds)

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

func (d *Dispatcher) deleteMessage(ctx context.Context, queueURL string, receiptHandle *string) {
	_, err := d.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: receiptHandle,
	})
	if err != nil {
		log.Printf("Failed to delete message from queue: %v", err)
	}
}

// Start begins a persistent long-polling loop against the SQS queues
func (d *Dispatcher) Start(ctx context.Context) {
	log.Println("Dispatcher worker starting...")

	// Background goroutine: Event-Driven TTL consumer
	go d.ttlConsumer(ctx)

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

type TTLMessage struct {
	ResourceID string `json:"resource_id"`
	RequestID  string `json:"request_id"`
}

func (d *Dispatcher) scheduleTTLRelease(ctx context.Context, resourceID, requestID string, delaySeconds int) {
	msgBody, _ := json.Marshal(TTLMessage{ResourceID: resourceID, RequestID: requestID})
	_, err := d.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:     aws.String(d.ttlQueueURL),
		MessageBody:  aws.String(string(msgBody)),
		DelaySeconds: int32(delaySeconds),
	})
	if err != nil {
		log.Printf("Failed to schedule TTL release for resource %s: %v", resourceID, err)
	}
}

// ttlConsumer long-polls the TTL queue for exact-time release events
func (d *Dispatcher) ttlConsumer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgResult, err := d.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(d.ttlQueueURL),
				MaxNumberOfMessages: 1,
				WaitTimeSeconds:     10,
			})
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			if len(msgResult.Messages) == 0 {
				continue
			}

			msg := msgResult.Messages[0]
			var ttlMsg TTLMessage
			if err := json.Unmarshal([]byte(*msg.Body), &ttlMsg); err != nil {
				d.deleteMessage(ctx, d.ttlQueueURL, msg.ReceiptHandle)
				continue
			}

			err = domain.ReleaseSpecificHold(ctx, d.dbClient, ttlMsg.ResourceID, ttlMsg.RequestID)
			if err != nil {
				log.Printf("Error releasing specific hold %s: %v", ttlMsg.ResourceID, err)
			} else {
				// No error means it was released or already taken care of. Either way, log a successful release attempt.
				// (The function returns nil if ConditionCheckFailed, meaning no-op).
				log.Printf("🔄 TTL exact event: Processed TTL for resource '%s' (Req: %s)", ttlMsg.ResourceID, ttlMsg.RequestID)
			}

			d.deleteMessage(ctx, d.ttlQueueURL, msg.ReceiptHandle)
		}
	}
}
