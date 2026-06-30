package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// handler is our new entry point. It replaces the http.HandlerFunc.
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	start := time.Now()
	requestID := requestIDFromContext(ctx, request)

	// Parse parameters from the Lambda event's query string.
	cfg := newConfigFromRequest(request.QueryStringParameters)
	baseFields := mergeLogFields(configLogFields(cfg), logFields{
		"requestId": requestID,
	})
	logEvent("info", "render request received", baseFields)

	if err := validateConfig(cfg); err != nil {
		durationMs := time.Since(start).Milliseconds()
		fields := mergeLogFields(baseFields, logFields{
			"durationMs": durationMs,
			"statusCode": 400,
			"error":      err.Error(),
		})
		logEvent("warn", "render validation failed", fields)
		emitRenderMetrics(mergeLogFields(fields, logFields{
			"RenderDurationMs":        durationMs,
			"RenderSuccess":           0,
			"RenderFailure":           0,
			"RenderValidationFailure": 1,
		}))

		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Headers: map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			},
			Body: err.Error(),
		}, nil
	}

	// Generate the raw pixel data.
	pixelBytes, err := generateFractalBytes(cfg)
	if err != nil {
		durationMs := time.Since(start).Milliseconds()
		fields := mergeLogFields(baseFields, logFields{
			"durationMs": durationMs,
			"statusCode": 500,
			"error":      err.Error(),
		})
		logEvent("error", "render failed", fields)
		emitRenderMetrics(mergeLogFields(fields, logFields{
			"RenderDurationMs":        durationMs,
			"RenderSuccess":           0,
			"RenderFailure":           1,
			"RenderValidationFailure": 0,
		}))

		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	// For binary responses, API Gateway requires the body to be Base64 encoded
	// and the IsBase64Encoded flag to be set to true.
	encodedBody := base64.StdEncoding.EncodeToString(pixelBytes)

	durationMs := time.Since(start).Milliseconds()
	fields := mergeLogFields(baseFields, logFields{
		"durationMs": durationMs,
		"statusCode": 200,
		"bytes":      len(pixelBytes),
	})
	logEvent("info", "render succeeded", fields)
	emitRenderMetrics(mergeLogFields(fields, logFields{
		"RenderDurationMs":        durationMs,
		"RenderSuccess":           1,
		"RenderFailure":           0,
		"RenderValidationFailure": 0,
	}))

	// Return the response.
	return events.APIGatewayProxyResponse{
		StatusCode:      200,
		Headers:         map[string]string{"Content-Type": "application/octet-stream"},
		Body:            encodedBody,
		IsBase64Encoded: true,
	}, nil
}

func main() {
	if addr := os.Getenv("MANDELBROT_LOCAL_HTTP_ADDR"); addr != "" {
		log.Fatal(startLocalHTTPServer(addr))
	}

	// This is the magic that connects our handler to the Lambda runtime.
	switch currentHandlerMode() {
	case renderModeWorker:
		lambda.Start(workerHandler)
	case renderModeOrchestrator:
		lambda.Start(orchestratorHandler)
	default:
		lambda.Start(handler)
	}
}

func currentHandlerMode() string {
	mode := os.Getenv("MANDELBROT_HANDLER_MODE")
	if mode == "" {
		return renderModeSingle
	}
	return mode
}
