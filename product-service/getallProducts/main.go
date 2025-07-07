package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-redis/redis/v8"

	"gitlab.connectwisedev.com/product-service/models"
	"gitlab.connectwisedev.com/product-service/pkg/cache"
	"gitlab.connectwisedev.com/product-service/pkg/config"
	"gitlab.connectwisedev.com/product-service/pkg/database"
)

var (
	dbClient    *database.DBClient
	redisClient *cache.RedisClient
	ctx         = context.Background()
)

func init() {
	config.LoadEnv() // Load environment variables first

	var err error
	dbClient, err = database.NewPostgresClient()
	if err != nil {
		log.Fatalf("Failed to initialize DB client: %v", err)
	}

	redisClient, err = cache.NewRedisClient()
	if err != nil {
		log.Fatalf("Failed to initialize Redis client: %v", err)
	}
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Received request: %v", request.Path)

	products, err := getProductsFromCache()
	if err != nil {
		log.Printf("Error fetching from Redis (%v), falling back to DB.", err)
		products, err = getProductsFromDB()
		if err != nil {
			log.Printf("Error fetching from DB: %v", err)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"message": "Failed to retrieve products"}`,
			}, nil
		}
		// If fetched from DB, try to populate cache for next time.
		// Run as a goroutine to not block the main request path.
		go func() {
			err := populateCache(products)
			if err != nil {
				log.Printf("Failed to populate cache after DB fetch: %v", err)
			}
		}()
	}

	responseBody, err := json.Marshal(products)
	if err != nil {
		log.Printf("Error marshaling products to JSON: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       `{"message": "Failed to format response"}`,
		}, nil
	}

	// Add appropriate caching headers
	headers := map[string]string{
		"Content-Type":                 "application/json",
		"Cache-Control":                "public, max-age=300, must-revalidate", // Cache for 5 minutes, revalidate after
		"Access-Control-Allow-Origin":  "*",                                    // Or specific domain for CORS, e.g., "https://example.com"
		"Access-Control-Allow-Methods": "GET",
		"Access-Control-Allow-Headers": "Content-Type",
	}

	// Add an ETag for conditional requests if you implement that logic
	// In a real system, you might hash the product list or use a version number for ETag
	// headers["ETag"] = `"some-hash-of-products"`

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       string(responseBody),
	}, nil
}

func getProductsFromCache() ([]models.Product, error) {
	// Get all product IDs from the Redis set
	productIDs, err := redisClient.GetClient().SMembers(ctx, "all_product_ids").Result()
	if err != nil {
		if err == redis.Nil { // Set does not exist
			return nil, fmt.Errorf("Redis set 'all_product_ids' does not exist or is empty")
		}
		return nil, fmt.Errorf("failed to get all_product_ids from Redis: %w", err)
	}
	if len(productIDs) == 0 {
		return nil, fmt.Errorf("no product IDs found in Redis cache set")
	}

	// Create keys for MGET
	keys := make([]string, len(productIDs))
	for i, id := range productIDs {
		keys[i] = fmt.Sprintf("product:%s", id)
	}

	// Fetch all product JSONs using MGET for efficiency
	results, err := redisClient.GetClient().MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to MGET products from Redis: %w", err)
	}

	var products []models.Product
	for _, res := range results {
		if res == nil {
			// This can happen if a key expired or was evicted from Redis.
			// Log and continue, as the DB fallback will cover it.
			log.Println("Found nil result for a product key in Redis, likely evicted/expired. Will re-fetch from DB if full cache miss.")
			continue
		}
		productJSON, ok := res.(string)
		if !ok {
			log.Printf("Unexpected type from Redis MGET: %T", res)
			continue
		}
		var product models.Product
		err := json.Unmarshal([]byte(productJSON), &product)
		if err != nil {
			log.Printf("Failed to unmarshal product JSON from Redis: %v", err)
			continue
		}
		products = append(products, product)
	}

	if len(products) == 0 && len(productIDs) > 0 {
		// This means we had product IDs, but couldn't retrieve or unmarshal any actual products.
		return nil, fmt.Errorf("all products from cache were invalid or missing after retrieval, forcing DB fetch")
	}

	log.Printf("Successfully retrieved %d products from Redis cache.", len(products))
	return products, nil
}

func getProductsFromDB() ([]models.Product, error) {
	rows, err := dbClient.GetDB().QueryContext(ctx, `SELECT id, name, image, price, qty, out_of_stock, created_at, updated_at FROM products ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query products from DB: %w", err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		var imageSQL sql.NullString // Use sql.NullString for nullable columns
		if err := rows.Scan(&p.ID, &p.Name, &imageSQL, &p.Price, &p.Qty, &p.OutOfStock, &p.CreatedAt, &p.UpdatedAt); err != nil {
			log.Printf("Error scanning product row from DB: %v", err)
			continue
		}
		if imageSQL.Valid {
			p.Image = &imageSQL.String
		}
		products = append(products, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration from DB: %w", err)
	}

	log.Printf("Successfully retrieved %d products from PostgreSQL database.", len(products))
	return products, nil
}

// populateCache clears and then re-populates the entire 'all_product_ids' set and individual product keys
func populateCache(products []models.Product) error {
	pipe := redisClient.GetClient().Pipeline()
	allProductIDs := make([]interface{}, len(products)) // To store IDs for SADD

	// Add/Update individual product entries and collect their IDs
	for i, p := range products {
		productJSON, err := json.Marshal(p)
		if err != nil {
			log.Printf("Failed to marshal product %s for cache population: %v", p.ID, err)
			continue
		}
		// Set a TTL (e.g., 5 minutes) for individual product keys.
		// This helps with eventual consistency if a product is deleted/changed by other means.
		pipe.Set(ctx, fmt.Sprintf("product:%s", p.ID), productJSON, 5*time.Minute)
		allProductIDs[i] = p.ID
	}

	// Clear existing product IDs set and add new ones to ensure consistency.
	// This is the most straightforward way to ensure 'all_product_ids' accurately reflects the DB.
	pipe.Del(ctx, "all_product_ids") // Remove old set of product IDs
	if len(allProductIDs) > 0 {
		pipe.SAdd(ctx, "all_product_ids", allProductIDs...) // Add all current product IDs
	}

	_, err := pipe.Exec(ctx) // Execute all pipeline commands
	if err != nil {
		return fmt.Errorf("failed to execute Redis pipeline for cache population: %w", err)
	}
	log.Printf("Cache populated with %d products.", len(products))
	return nil
}

func main() {
	defer dbClient.Close()
	defer redisClient.Close()
	lambda.Start(handler)
}
