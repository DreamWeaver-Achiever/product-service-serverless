package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// LoadEnv loads environment variables from .env.local if APP_ENV is "local"
func LoadEnv() {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development" // Default to development if not set
		os.Setenv("APP_ENV", appEnv)
	}

	if appEnv == "local" {
		err := godotenv.Load(".env.local") // Assumes .env.local exists in root or where app is run
		if err != nil {
			log.Printf("Warning: .env.local file not found, or error loading: %v. Relying on system environment variables.", err)
		} else {
			log.Println("Loaded .env.local for local development.")
		}
	} else {
		log.Printf("Running in %s environment. Not loading .env.local.", appEnv)
	}
}
