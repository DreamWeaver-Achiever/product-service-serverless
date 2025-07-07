package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"

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

// S3EventWrapper is a custom struct to handle either S3 events or direct CSV payload
type S3EventWrapper struct {
	Records []events.S3EventRecord `json:"Records,omitempty"`
	CSVData string                 `json:"csv_data,omitempty"` // For local testing
}

func handler(event S3EventWrapper) error {
	var csvContent []byte
	var err error

	if len(event.Records) > 0 {
		// This path is for S3 event triggers (production).
		// In a real scenario, you'd download the CSV from S3 here using AWS SDK for Go.
		// For this assessment, if triggered by S3, we'll simulate by reading a local file IF in local env.
		// Otherwise, it will fail, prompting you to implement real S3 download.

		s3Record := event.Records[0].S3
		bucketName := s3Record.Bucket.Name
		key := s3Record.Object.Key

		log.Printf("Processing S3 event for bucket: %s, key: %s", bucketName, key)

		if os.Getenv("APP_ENV") == "local" {
			log.Println("Running in local environment, attempting to read local CSV from 'products.csv' for S3 simulation.")
			csvContent, err = os.ReadFile("products.csv") // Assume products.csv exists in the root for local testing
			if err != nil {
				return fmt.Errorf("failed to read local products.csv for S3 simulation: %w", err)
			}
		} else {
			// **IMPORTANT:** For actual AWS deployment with S3 trigger,
			// you must uncomment and implement AWS SDK S3 GetObject here.
			// Example:
			// sess, _ := session.NewSession()
			// svc := s3.New(sess)
			// result, err := svc.GetObject(&s3.GetObjectInput{
			// 	Bucket: aws.String(bucketName),
			// 	Key:    aws.String(key),
			// })
			// if err != nil {
			// 	return fmt.Errorf("failed to get object from S3: %w", err)
			// }
			// defer result.Body.Close()
			// csvContent, err = io.ReadAll(result.Body)
			// if err != nil {
			// 	return fmt.Errorf("failed to read S3 object body: %w", err)
			// }
			return fmt.Errorf("S3 event triggered, but S3 download logic is not implemented for non-local environment in this example. Please integrate AWS SDK for S3 if deploying to real AWS.")
		}
	} else if event.CSVData != "" {
		// This path is for direct invocation with CSV data (for local testing via Postman/CLI)
		log.Println("Processing direct CSV data payload.")
		csvContent = []byte(event.CSVData)
	} else {
		return fmt.Errorf("no S3 event record or direct CSV data found in the payload")
	}

	reader := csv.NewReader(bytes.NewReader(csvContent))
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("CSV is empty or has only headers")
	}

	// header := records[0] // If you need to validate headers explicitly
	dataRows := records[1:]

	tx, err := dbClient.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback on error by default

	for i, row := range dataRows {
		if len(row) < 5 { // Basic validation: check minimum number of columns (id, name, image, price, qty)
			log.Printf("Skipping row %d due to insufficient columns: %v", i+2, row)
			continue
		}

		productCSV := models.ProductCSV{}
		// Map CSV columns to struct fields - adjust indices based on your CSV structure
		// Assuming order: id, name, image, price, qty
		productCSV.ID = row[0] // If CSV provides ID
		productCSV.Name = row[1]
		productCSV.Image = row[2] // This can be empty string for NULL

		productCSV.Price, err = strconv.ParseFloat(row[3], 64)
		if err != nil {
			log.Printf("Skipping row %d: Invalid price '%s': %v", i+2, row[3], err)
			continue
		}

		productCSV.Qty, err = strconv.Atoi(row[4])
		if err != nil {
			log.Printf("Skipping row %d: Invalid quantity '%s': %v", i+2, row[4], err)
			continue
		}

		// Handle ID: If CSV provides ID, use it. Otherwise, generate a new one.
		productID := productCSV.ID
		if productID == "" {
			productID = uuid.New().String()
		}

		// UPSERT into PostgreSQL
		_, err = tx.ExecContext(ctx, `
			INSERT INTO products (id, name, image, price, qty)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (name) DO UPDATE SET
				image = EXCLUDED.image,
				price = EXCLUDED.price,
				qty = EXCLUDED.qty,
				updated_at = NOW(),
				out_of_stock = (EXCLUDED.qty = 0) -- Trigger handles this, but explicit here for clarity/robustness
			RETURNING id;
		`, productID, productCSV.Name, nullString(productCSV.Image), productCSV.Price, productCSV.Qty)
		if err != nil {
			log.Printf("Error processing product %s (row %d) for DB UPSERT: %v", productCSV.Name, i+2, err)
			continue // Continue processing other rows even if one fails
		}

		// Re-fetch the product from DB to ensure we have the correct ID, created_at, updated_at, out_of_stock status
		// This is important because ID might be generated or fetched by `RETURNING id`.
		var storedProduct models.Product
		var imageSQL sql.NullString
		rowDB := tx.QueryRowContext(ctx, `SELECT id, name, image, price, qty, out_of_stock, created_at, updated_at FROM products WHERE name = $1`, productCSV.Name)
		err = rowDB.Scan(&storedProduct.ID, &storedProduct.Name, &imageSQL, &storedProduct.Price, &storedProduct.Qty, &storedProduct.OutOfStock, &storedProduct.CreatedAt, &storedProduct.UpdatedAt)
		if err != nil {
			log.Printf("Error re-fetching product %s for cache update: %v", productCSV.Name, err)
			continue
		}
		if imageSQL.Valid {
			storedProduct.Image = &imageSQL.String
		}

		productJSON, err := json.Marshal(storedProduct)
		if err != nil {
			log.Printf("Error marshaling product %s to JSON for Redis: %v", storedProduct.Name, err)
			continue
		}

		// Update Redis cache for this product (individual key)
		err = redisClient.GetClient().Set(ctx, fmt.Sprintf("product:%s", storedProduct.ID), productJSON, 0).Err() // No expiration
		if err != nil {
			log.Printf("Error setting product %s in Redis: %v", storedProduct.Name, err)
			// This is a soft failure for Redis, continue processing DB
		}

		// Add product ID to a set for easy retrieval of all product IDs in getAllProducts
		err = redisClient.GetClient().SAdd(ctx, "all_product_ids", storedProduct.ID).Err()
		if err != nil {
			log.Printf("Error adding product ID %s to all_product_ids set in Redis: %v", storedProduct.ID, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Println("Products processed successfully and cache updated.")
	return nil
}

// nullString converts a Go string to sql.NullString for nullable DB columns
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func main() {
	defer dbClient.Close()
	defer redisClient.Close()
	lambda.Start(handler)
}
