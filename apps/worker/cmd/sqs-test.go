package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func main() {
	log.Println("Starting SQS connectivity test...")

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

	// Load AWS SDK configuration
	var configOptions []func(*config.LoadOptions) error
	configOptions = append(configOptions, config.WithRegion(region))

	// Set custom endpoint for LocalStack
	if useLocalStack && endpoint != "" {
		configOptions = append(configOptions, config.WithBaseEndpoint(endpoint))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), configOptions...)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create an SQS client
	client := sqs.NewFromConfig(cfg)

	// Get queue URL
	queueURLOutput, err := client.GetQueueUrl(context.TODO(), &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})

	if err != nil {
		log.Fatalf("Failed to get queue URL: %v", err)
	}

	queueURL := queueURLOutput.QueueUrl
	log.Printf("SQS Queue URL: %s", *queueURL)

	// Try to send a test message
	messageBody := fmt.Sprintf("Test message sent at %s", time.Now().Format(time.RFC3339))
	_, err = client.SendMessage(context.TODO(), &sqs.SendMessageInput{
		QueueUrl:    queueURL,
		MessageBody: aws.String(messageBody),
	})

	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	log.Printf("Successfully sent message: %s", messageBody)

	// Try to receive the message
	receiveOutput, err := client.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
		QueueUrl:            queueURL,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     5,
	})

	if err != nil {
		log.Fatalf("Failed to receive message: %v", err)
	}

	if len(receiveOutput.Messages) == 0 {
		log.Println("No messages received within timeout")
	} else {
		log.Printf("Received message: %s", *receiveOutput.Messages[0].Body)

		// Delete the message
		_, err = client.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
			QueueUrl:      queueURL,
			ReceiptHandle: receiveOutput.Messages[0].ReceiptHandle,
		})

		if err != nil {
			log.Fatalf("Failed to delete message: %v", err)
		}

		log.Println("Successfully deleted message")
	}

	log.Println("SQS connectivity test completed successfully!")
}
