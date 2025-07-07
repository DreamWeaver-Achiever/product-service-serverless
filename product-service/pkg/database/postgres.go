package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// DBClient holds the PostgreSQL database connection
type DBClient struct {
	db *sql.DB
}

// NewPostgresClient initializes and returns a new PostgreSQL client
func NewPostgresClient() (*DBClient, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL!")
	return &DBClient{db: db}, nil
}

// Close closes the database connection
func (c *DBClient) Close() {
	if c.db != nil {
		c.db.Close()
		log.Println("PostgreSQL connection closed.")
	}
}

// GetDB returns the underlying *sql.DB instance
func (c *DBClient) GetDB() *sql.DB {
	return c.db
}
