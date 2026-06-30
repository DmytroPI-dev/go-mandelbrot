package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func validWorkerRenderRequest() WorkerRenderRequest {
	return WorkerRenderRequest{
		RequestID: "render-1",
		Image: WorkerImage{
			Width:      6,
			HeightPx:   4,
			PosX:       -2,
			PosY:       -1.25,
			ViewHeight: 2.5,
			Samples:    2,
			MaxIter:    25,
		},
		Tile: Tile{
			X:        1,
			Y:        1,
			Width:    3,
			HeightPx: 2,
		},
	}
}

func TestConfigFromWorkerImage(t *testing.T) {
	request := validWorkerRenderRequest()

	cfg := configFromWorkerImage(request.Image)

	if cfg.imgWidth != request.Image.Width || cfg.imgHeight != request.Image.HeightPx {
		t.Fatalf("image size = %dx%d, want %dx%d", cfg.imgWidth, cfg.imgHeight, request.Image.Width, request.Image.HeightPx)
	}
	if cfg.posX != request.Image.PosX || cfg.posY != request.Image.PosY || cfg.height != request.Image.ViewHeight {
		t.Fatalf("viewport = (%v,%v,%v), want (%v,%v,%v)", cfg.posX, cfg.posY, cfg.height, request.Image.PosX, request.Image.PosY, request.Image.ViewHeight)
	}
	if cfg.samples != request.Image.Samples || cfg.maxIter != request.Image.MaxIter {
		t.Fatalf("render controls = samples:%d maxIter:%d", cfg.samples, cfg.maxIter)
	}
	if cfg.numBlocks != 1 || cfg.numThreads != 1 {
		t.Fatalf("worker config = blocks:%d threads:%d, want 1 and 1", cfg.numBlocks, cfg.numThreads)
	}
	if cfg.pixelTotal != request.Image.Width*request.Image.HeightPx {
		t.Fatalf("pixelTotal = %d, want %d", cfg.pixelTotal, request.Image.Width*request.Image.HeightPx)
	}
}

func TestWorkerHandlerReturnsBase64EncodedTileBytes(t *testing.T) {
	request := validWorkerRenderRequest()
	cfg := configFromWorkerImage(request.Image)
	wantBytes, err := renderTileBytes(cfg, request.Tile)
	if err != nil {
		t.Fatalf("renderTileBytes returned error: %v", err)
	}

	resp, err := workerHandler(context.Background(), request)
	if err != nil {
		t.Fatalf("workerHandler returned error: %v", err)
	}
	if resp.RequestID != request.RequestID {
		t.Fatalf("requestId = %q, want %q", resp.RequestID, request.RequestID)
	}
	if resp.Encoding != "rgba" {
		t.Fatalf("encoding = %q, want rgba", resp.Encoding)
	}
	if resp.ByteLength != len(wantBytes) {
		t.Fatalf("byteLength = %d, want %d", resp.ByteLength, len(wantBytes))
	}
	if resp.Tile != request.Tile {
		t.Fatalf("tile = %+v, want %+v", resp.Tile, request.Tile)
	}

	gotBytes, err := base64.StdEncoding.DecodeString(resp.BytesBase64)
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatal("workerHandler bytes do not match renderTileBytes")
	}
}

func TestWorkerHandlerRejectsInvalidTile(t *testing.T) {
	request := validWorkerRenderRequest()
	request.Tile = Tile{X: 5, Y: 3, Width: 2, HeightPx: 2}

	_, err := workerHandler(context.Background(), request)
	if err == nil {
		t.Fatal("workerHandler returned nil error, want tile validation error")
	}
	if !strings.Contains(err.Error(), "exceeds image bounds") {
		t.Fatalf("error = %q, want image bounds message", err.Error())
	}
}

func TestWorkerHandlerEmitsValidationLogsAndMetrics(t *testing.T) {
	buf := captureLogs(t)
	request := validWorkerRenderRequest()
	request.Image.MaxIter = 0

	_, err := workerHandler(context.Background(), request)
	if err == nil {
		t.Fatal("workerHandler returned nil error, want validation error")
	}

	entries := decodeLogs(t, buf)
	if len(entries) != 3 {
		t.Fatalf("log entries = %d, want 3", len(entries))
	}

	validationLog := entries[1]
	if validationLog["message"] != "tile render validation failed" {
		t.Fatalf("validation message = %v, want tile render validation failed", validationLog["message"])
	}
	if validationLog["mode"] != renderModeWorker {
		t.Fatalf("mode = %v, want %s", validationLog["mode"], renderModeWorker)
	}

	metricsLog := entries[2]
	if metricsLog["mode"] != renderModeWorker {
		t.Fatalf("metric mode = %v, want %s", metricsLog["mode"], renderModeWorker)
	}
	if metricsLog["RenderValidationFailure"] != float64(1) {
		t.Fatalf("RenderValidationFailure = %v, want 1", metricsLog["RenderValidationFailure"])
	}
}
