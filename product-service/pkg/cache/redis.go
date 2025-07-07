package cache

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient holds the Redis client connection
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient initializes and returns a new Redis client
func NewRedisClient() (*RedisClient, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return nil, fmt.Errorf("REDIS_ADDR environment variable not set")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // No password by default for local Redis
		DB:       0,  // Default DB
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pong, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Printf("Successfully connected to Redis! Ping response: %s", pong)

	return &RedisClient{client: client}, nil
}

// Close closes the Redis connection
func (c *RedisClient) Close() {
	if c.client != nil {
		c.client.Close()
		log.Println("Redis connection closed.")
	}
}

// GetClient returns the underlying *redis.Client instance
func (c *RedisClient) GetClient() *redis.Client {
	return c.client
}
