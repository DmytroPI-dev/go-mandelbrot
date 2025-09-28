package main

import (
	"context"
	"encoding/base64"
	"log"
	"time"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// handler is our new entry point. It replaces the http.HandlerFunc.
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	start := time.Now()

	// Parse parameters from the Lambda event's query string.
	cfg := newConfigFromRequest(request.QueryStringParameters)
	log.Printf("Handling request with config: %+v", cfg)

	// Generate the raw pixel data.
	pixelBytes, err := generateFractalBytes(cfg)
	if err != nil {
		log.Printf("Error generating fractal: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	// For binary responses, API Gateway requires the body to be Base64 encoded
	// and the IsBase64Encoded flag to be set to true.
	encodedBody := base64.StdEncoding.EncodeToString(pixelBytes)

	log.Printf("Finished generation in %v. Sending %d bytes.", time.Since(start), len(pixelBytes))

	// Return the response.
	return events.APIGatewayProxyResponse{
		StatusCode:      200,
		Headers:         map[string]string{"Content-Type": "application/octet-stream"},
		Body:            encodedBody,
		IsBase64Encoded: true,
	}, nil
}

func main() {
	// This is the magic that connects our handler to the Lambda runtime.
	lambda.Start(handler)
}