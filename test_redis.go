package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Get Redis host and port from environment variables
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	
	address := fmt.Sprintf("%s:%s", redisHost, redisPort)
	fmt.Printf("Connecting to Redis at %s\n", address)
	
	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: "", // no password
		DB:       0,  // use default DB
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Test connection
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	
	fmt.Printf("Redis connection successful: %s\n", pong)
	
	// Set a value
	err = rdb.Set(ctx, "test_key", "test_value", 0).Err()
	if err != nil {
		log.Fatalf("Failed to set key: %v", err)
	}
	
	// Get the value
	val, err := rdb.Get(ctx, "test_key").Result()
	if err != nil {
		log.Fatalf("Failed to get key: %v", err)
	}
	
	fmt.Printf("Retrieved value from Redis: %s\n", val)
}
