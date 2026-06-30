package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func validOrchestratorRequest() events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		RequestContext: events.APIGatewayProxyRequestContext{
			RequestID: "distributed-1",
		},
		QueryStringParameters: map[string]string{
			"width":      "34",
			"height_px":  "18",
			"height":     "2.5",
			"samples":    "2",
			"maxIter":    "25",
			"numBlocks":  "4",
			"numThreads": "2",
			"tileSize":   "16",
		},
	}
}

func TestPlanValidatedTilesRejectsUnsafeTileSize(t *testing.T) {
	cfg := validTestConfig()

	_, err := planValidatedTiles(cfg, minTileSize-1)
	if err == nil {
		t.Fatal("planValidatedTiles returned nil error, want tileSize validation error")
	}
	if !strings.Contains(err.Error(), "tileSize") {
		t.Fatalf("error = %q, want tileSize message", err.Error())
	}
}

func TestPlanValidatedTilesRejectsTooManyTiles(t *testing.T) {
	cfg := validTestConfig()
	cfg.imgWidth = maxImageDimension
	cfg.imgHeight = maxImageDimension
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight

	_, err := planValidatedTiles(cfg, minTileSize)
	if err == nil {
		t.Fatal("planValidatedTiles returned nil error, want tile count validation error")
	}
	if !strings.Contains(err.Error(), "tile count") {
		t.Fatalf("error = %q, want tile count message", err.Error())
	}
}

func TestOrchestratorHandlerReturnsFullRGBABytes(t *testing.T) {
	request := validOrchestratorRequest()
	cfg := newConfigFromRequest(request.QueryStringParameters)
	wantBytes, err := generateFractalBytes(cfg)
	if err != nil {
		t.Fatalf("generateFractalBytes returned error: %v", err)
	}

	resp, err := orchestratorHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("orchestratorHandler returned error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !resp.IsBase64Encoded {
		t.Fatal("IsBase64Encoded = false, want true")
	}
	if resp.Headers["Content-Type"] != "application/octet-stream" {
		t.Fatalf("content type = %q, want application/octet-stream", resp.Headers["Content-Type"])
	}

	gotBytes, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	if len(gotBytes) != len(wantBytes) {
		t.Fatalf("decoded length = %d, want %d", len(gotBytes), len(wantBytes))
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatal("orchestrator bytes do not match full render bytes")
	}
}

func TestOrchestratorHandlerRejectsTooManyTiles(t *testing.T) {
	request := validOrchestratorRequest()
	request.QueryStringParameters["width"] = "1200"
	request.QueryStringParameters["height_px"] = "1200"
	request.QueryStringParameters["tileSize"] = "16"

	resp, err := orchestratorHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("orchestratorHandler returned error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, "tile count") {
		t.Fatalf("body = %q, want tile count message", resp.Body)
	}
}

func TestOrchestratorHandlerEmitsValidationLogsAndMetrics(t *testing.T) {
	buf := captureLogs(t)
	request := validOrchestratorRequest()
	request.QueryStringParameters["maxIter"] = "0"

	resp, err := orchestratorHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("orchestratorHandler returned error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	entries := decodeLogs(t, buf)
	if len(entries) != 3 {
		t.Fatalf("log entries = %d, want 3", len(entries))
	}

	validationLog := entries[1]
	if validationLog["message"] != "distributed render validation failed" {
		t.Fatalf("validation message = %v, want distributed render validation failed", validationLog["message"])
	}
	if validationLog["mode"] != renderModeOrchestrator {
		t.Fatalf("mode = %v, want %s", validationLog["mode"], renderModeOrchestrator)
	}

	metricsLog := entries[2]
	if metricsLog["mode"] != renderModeOrchestrator {
		t.Fatalf("metric mode = %v, want %s", metricsLog["mode"], renderModeOrchestrator)
	}
	if metricsLog["RenderValidationFailure"] != float64(1) {
		t.Fatalf("RenderValidationFailure = %v, want 1", metricsLog["RenderValidationFailure"])
	}
}
