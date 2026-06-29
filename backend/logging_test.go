package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}

	logMu.Lock()
	previous := logOutput
	logOutput = buf
	logMu.Unlock()

	t.Cleanup(func() {
		logMu.Lock()
		logOutput = previous
		logMu.Unlock()
	})

	return buf
}

func decodeLogs(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	var entries []map[string]any
	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("Decode log entry returned error: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestLogEventWritesStructuredJSON(t *testing.T) {
	buf := captureLogs(t)

	logEvent("info", "test event", logFields{
		"requestId": "request-1",
		"level":     "should-not-override",
	})

	entries := decodeLogs(t, buf)
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}

	entry := entries[0]
	if entry["level"] != "info" {
		t.Fatalf("level = %v, want info", entry["level"])
	}
	if entry["message"] != "test event" {
		t.Fatalf("message = %v, want test event", entry["message"])
	}
	if entry["requestId"] != "request-1" {
		t.Fatalf("requestId = %v, want request-1", entry["requestId"])
	}
	if _, ok := entry["timestamp"].(string); !ok {
		t.Fatalf("timestamp = %v, want string", entry["timestamp"])
	}
}

func TestHandlerEmitsValidationLogsAndMetrics(t *testing.T) {
	buf := captureLogs(t)

	resp, err := handler(context.Background(), events.APIGatewayProxyRequest{
		RequestContext: events.APIGatewayProxyRequestContext{
			RequestID: "request-2",
		},
		QueryStringParameters: map[string]string{"maxIter": "0"},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	entries := decodeLogs(t, buf)
	if len(entries) != 3 {
		t.Fatalf("log entries = %d, want 3", len(entries))
	}

	validationLog := entries[1]
	if validationLog["message"] != "render validation failed" {
		t.Fatalf("validation message = %v, want render validation failed", validationLog["message"])
	}
	if validationLog["requestId"] != "request-2" {
		t.Fatalf("requestId = %v, want request-2", validationLog["requestId"])
	}
	if validationLog["statusCode"] != float64(400) {
		t.Fatalf("statusCode = %v, want 400", validationLog["statusCode"])
	}

	metricsLog := entries[2]
	if _, ok := metricsLog["_aws"].(map[string]any); !ok {
		t.Fatalf("_aws = %v, want embedded metric metadata", metricsLog["_aws"])
	}
	if metricsLog["RenderValidationFailure"] != float64(1) {
		t.Fatalf("RenderValidationFailure = %v, want 1", metricsLog["RenderValidationFailure"])
	}
	if metricsLog["RenderFailure"] != float64(0) {
		t.Fatalf("RenderFailure = %v, want 0", metricsLog["RenderFailure"])
	}
	if metricsLog["RenderSuccess"] != float64(0) {
		t.Fatalf("RenderSuccess = %v, want 0", metricsLog["RenderSuccess"])
	}
}
