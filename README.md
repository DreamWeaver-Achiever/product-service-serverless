Product Service - Serverless Application

Welcome to the Product Service - Serverless Application! This project delivers a robust backend solution for product management, leveraging the power of AWS Serverless Application Model (SAM) and Go. It's designed for scalability and efficiency, integrating seamlessly with PostgreSQL for persistent storage and Redis for high-speed caching.

🚀 Project Overview

This application provides a flexible and performant way to manage product data. Whether you're fetching product listings or handling bulk uploads, this serverless architecture ensures reliability and responsiveness.

✨ Key Features

Get All Products API: A straightforward HTTP GET endpoint (/products) to fetch all product data, directly powered by your PostgreSQL database.

Product Upload Function: A dedicated Lambda function built to efficiently process CSV files (e.g., from an S3 bucket). It handles bulk updates for your product data in both PostgreSQL and Redis, ensuring your database and cache are always in sync. (Note: S3 integration is fully functional upon cloud deployment).

Robust Database Integration: Utilizes PostgreSQL as the reliable backbone for all your persistent product data storage.

High-Performance Caching Layer: Integrates Redis to provide a lightning-fast caching mechanism, drastically speeding up data retrieval for frequently accessed product information.

Seamless Local Development: Designed with developers in mind! Enjoy a smooth local development experience with Docker Compose for managing your PostgreSQL and Redis instances, and AWS SAM CLI for local Lambda function invocation and API simulation.

🛠️ Technology Stack

Go: The efficient and performant backend language.

AWS Lambda: Serverless compute for scalable function execution.

AWS API Gateway: The entry point for your API endpoints.

PostgreSQL: Your reliable relational database.

Redis: Your in-memory data store for caching.

Docker & Docker Compose: For containerized local services.

AWS SAM CLI: For simplified serverless application development and deployment.

🏃‍♀️ Getting Started Locally

Follow these simple steps to get the Product Service up and running on your local machine.

Prerequisites

Before you begin, ensure you have the following installed:

Go: Version 1.x or higher.

Docker and Docker Compose: For running local database and cache services.

AWS SAM CLI: Version 1.119.0 or higher is recommended.

Setup Instructions
Extract the project archive:

Bash

unzip product-service.zip
Navigate into the project directory:

Bash

cd product-service
Start PostgreSQL and Redis with Docker Compose:
This will spin up your database and caching layers in the background.

Bash

docker compose up -d
Install Go module dependencies:

Bash

go mod tidy
Build the SAM project:
This step prepares your Lambda functions for local execution. Using --use-container ensures a consistent build environment.

Bash

sam build --use-container
Local API Testing
Once your project is built, you can start the local API Gateway to test your endpoints.

Start the local API Gateway:

Bash

sam local start-api
You'll see output indicating the API is running, typically at http://127.0.0.1:3000.

Test the Get All Products API:
Open a new terminal window (keep sam local start-api running in the first) and execute the following curl command:

Bash

curl http://127.0.0.1:3000/products
You should see an empty array [] if no products are in your database yet, or a list of products if you've added some!

Invoke the UploadProductFunction locally:
To test the product upload functionality, you can invoke the Lambda function directly using a sample payload. Ensure you have a direct-csv-payload.json file in your project root with the necessary CSV data.

Bash

sam local invoke UploadProductFunction -e direct-csv-payload.json
Feel free to explore the codebase and contribute! If you have any questions or run into issues, don't hesitate to reach out.
