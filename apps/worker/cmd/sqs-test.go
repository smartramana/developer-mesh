package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/cenkalti/backoff/v4"
)

func main() {
	log.Println("Starting SQS connectivity test...")

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, cancelling operations...")
		cancel()
	}()

	// Configure AWS SDK with LocalStack settings
	useLocalStack := os.Getenv("USE_LOCALSTACK") == "true"
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	region := os.Getenv("AWS_REGION")
	queueName := os.Getenv("SQS_QUEUE_NAME")

	if queueName == "" {
		queueName = "tasks"
	}

	log.Printf("SQS Settings: useLocalStack=%v, endpoint=%s, region=%s, queueName=%s",
		useLocalStack, endpoint, region, queueName)

	// Load AWS SDK configuration with timeout
	var configOptions []func(*config.LoadOptions) error
	configOptions = append(configOptions, config.WithRegion(region))

	// Set custom endpoint for LocalStack
	if useLocalStack && endpoint != "" {
		configOptions = append(configOptions, config.WithBaseEndpoint(endpoint))
	}

	configCtx, configCancel := context.WithTimeout(ctx, 10*time.Second)
	defer configCancel()

	cfg, err := config.LoadDefaultConfig(configCtx, configOptions...)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create an SQS client
	client := sqs.NewFromConfig(cfg)

	// Get queue URL with retry and timeout
	var queueURL *string
	queueURLOperation := func() error {
		queueCtx, queueCancel := context.WithTimeout(ctx, 5*time.Second)
		defer queueCancel()

		queueURLOutput, err := client.GetQueueUrl(queueCtx, &sqs.GetQueueUrlInput{
			QueueName: aws.String(queueName),
		})
		if err != nil {
			return fmt.Errorf("failed to get queue URL: %w", err)
		}
		queueURL = queueURLOutput.QueueUrl
		return nil
	}

	// Use exponential backoff for retries
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.MaxElapsedTime = 30 * time.Second
	
	if err := backoff.Retry(queueURLOperation, backoff.WithContext(backoffConfig, ctx)); err != nil {
		log.Fatalf("Failed to get queue URL after retries: %v", err)
	}

	log.Printf("SQS Queue URL: %s", *queueURL)

	// Try to send a test message with retry
	messageBody := fmt.Sprintf("Test message sent at %s", time.Now().Format(time.RFC3339))
	
	sendOperation := func() error {
		sendCtx, sendCancel := context.WithTimeout(ctx, 10*time.Second)
		defer sendCancel()
		
		_, err := client.SendMessage(sendCtx, &sqs.SendMessageInput{
			QueueUrl:    queueURL,
			MessageBody: aws.String(messageBody),
		})
		if err != nil {
			// Check if it's an AWS-specific error
			if isRetryableAWSError(err) {
				return fmt.Errorf("retryable error sending message: %w", err)
			}
			// Non-retryable error
			return backoff.Permanent(fmt.Errorf("permanent error sending message: %w", err))
		}
		return nil
	}

	backoffConfig = backoff.NewExponentialBackOff()
	backoffConfig.MaxElapsedTime = 30 * time.Second
	
	if err := backoff.Retry(sendOperation, backoff.WithContext(backoffConfig, ctx)); err != nil {
		log.Fatalf("Failed to send message after retries: %v", err)
	}

	log.Printf("Successfully sent message: %s", messageBody)

	// Try to receive the message with retry
	var messages []types.Message
	receiveOperation := func() error {
		receiveCtx, receiveCancel := context.WithTimeout(ctx, 30*time.Second)
		defer receiveCancel()
		
		output, err := client.ReceiveMessage(receiveCtx, &sqs.ReceiveMessageInput{
			QueueUrl:            queueURL,
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20, // Long polling
		})
		if err != nil {
			if isRetryableAWSError(err) {
				return fmt.Errorf("retryable error receiving message: %w", err)
			}
			return backoff.Permanent(fmt.Errorf("permanent error receiving message: %w", err))
		}
		messages = output.Messages
		return nil
	}

	backoffConfig = backoff.NewExponentialBackOff()
	backoffConfig.MaxElapsedTime = 60 * time.Second
	
	if err := backoff.Retry(receiveOperation, backoff.WithContext(backoffConfig, ctx)); err != nil {
		log.Printf("Failed to receive message after retries: %v", err)
		// Don't fatally exit here, as no messages is acceptable
	}

	if len(messages) == 0 {
		log.Println("No messages received within timeout")
	} else {
		log.Printf("Received message: %s", *messages[0].Body)

		// Delete the message with retry
		deleteOperation := func() error {
			deleteCtx, deleteCancel := context.WithTimeout(ctx, 5*time.Second)
			defer deleteCancel()
			
			_, err := client.DeleteMessage(deleteCtx, &sqs.DeleteMessageInput{
				QueueUrl:      queueURL,
				ReceiptHandle: messages[0].ReceiptHandle,
			})
			if err != nil {
				if isRetryableAWSError(err) {
					return fmt.Errorf("retryable error deleting message: %w", err)
				}
				return backoff.Permanent(fmt.Errorf("permanent error deleting message: %w", err))
			}
			return nil
		}

		backoffConfig = backoff.NewExponentialBackOff()
		backoffConfig.MaxElapsedTime = 15 * time.Second
		
		if err := backoff.Retry(deleteOperation, backoff.WithContext(backoffConfig, ctx)); err != nil {
			log.Printf("Failed to delete message after retries: %v", err)
		} else {
			log.Println("Successfully deleted message")
		}
	}

	log.Println("SQS connectivity test completed successfully!")
}

// isRetryableAWSError checks if an AWS error is retryable
func isRetryableAWSError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable error patterns
	errStr := err.Error()
	retryablePatterns := []string{
		"throttled",
		"TooManyRequestsException",
		"RequestLimitExceeded",
		"ServiceUnavailable",
		"RequestTimeout",
		"connection refused",
		"connection reset",
		"temporary failure",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

// findSubstring searches for a substring in a string
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
