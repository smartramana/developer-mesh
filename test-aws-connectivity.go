package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/sqs"
)

func main() {
    ctx := context.Background()
    
    // Load AWS config
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        fmt.Printf("❌ Failed to load AWS config: %v\n", err)
        return
    }
    
    fmt.Println("🔍 Testing AWS Connectivity...")
    fmt.Printf("   Region: %s\n", cfg.Region)
    fmt.Printf("   S3 Bucket: %s\n", os.Getenv("S3_BUCKET"))
    fmt.Printf("   SQS Queue: %s\n", os.Getenv("SQS_QUEUE_URL"))
    
    // Test S3
    s3Client := s3.NewFromConfig(cfg)
    bucket := os.Getenv("S3_BUCKET")
    if bucket != "" {
        _, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
            Bucket: &bucket,
        })
        if err != nil {
            fmt.Printf("❌ S3 bucket not accessible: %v\n", err)
        } else {
            fmt.Printf("✅ S3 bucket accessible: %s\n", bucket)
        }
    }
    
    // Test SQS
    sqsClient := sqs.NewFromConfig(cfg)
    queueURL := os.Getenv("SQS_QUEUE_URL")
    if queueURL != "" {
        _, err = sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
            QueueUrl: &queueURL,
        })
        if err != nil {
            fmt.Printf("❌ SQS queue not accessible: %v\n", err)
        } else {
            fmt.Printf("✅ SQS queue accessible: %s\n", queueURL)
        }
    }
    
    // Test Redis
    redisAddr := os.Getenv("REDIS_ADDR")
    fmt.Printf("\n🔍 Redis Configuration:\n")
    fmt.Printf("   REDIS_ADDR: %s\n", redisAddr)
    fmt.Printf("   Note: Redis in private subnet - needs SSH tunnel or VPN\n")
    
    // Test Database
    fmt.Printf("\n🔍 Database Configuration:\n")
    fmt.Printf("   DATABASE_HOST: %s\n", os.Getenv("DATABASE_HOST"))
    fmt.Printf("   DATABASE_PORT: %s\n", os.Getenv("DATABASE_PORT"))
    fmt.Printf("   DATABASE_NAME: %s\n", os.Getenv("DATABASE_NAME"))
    fmt.Printf("   Note: RDS in private subnet - needs SSH tunnel or VPN\n")
}