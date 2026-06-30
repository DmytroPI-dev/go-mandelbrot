package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

func orchestratorHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	start := time.Now()
	requestID := requestIDFromContext(ctx, request)
	cfg := newConfigFromRequest(request.QueryStringParameters)
	tileSize := getIntParam(request.QueryStringParameters, "tileSize", defaultTileSize)

	baseFields := mergeLogFields(orchestratorLogFields(cfg, tileSize, 0), logFields{
		"requestId": requestID,
	})
	logEvent("info", "distributed render request received", baseFields)

	if err := validateConfig(cfg); err != nil {
		return orchestratorValidationResponse(start, baseFields, err), nil
	}
	tiles, err := planValidatedTiles(cfg, tileSize)
	if err != nil {
		return orchestratorValidationResponse(start, baseFields, err), nil
	}
	baseFields = mergeLogFields(baseFields, logFields{"tileCount": len(tiles)})

	results := make([]TileResult, 0, len(tiles))
	for _, tile := range tiles {
		workerResp, err := workerHandler(ctx, WorkerRenderRequest{
			RequestID: requestID,
			Image:     workerImageFromConfig(cfg),
			Tile:      tile,
		})
		if err != nil {
			durationMs := time.Since(start).Milliseconds()
			fields := mergeLogFields(baseFields, logFields{
				"durationMs": durationMs,
				"error":      err.Error(),
			})
			logEvent("error", "distributed render failed", fields)
			emitRenderMetrics(mergeLogFields(fields, logFields{
				"RenderDurationMs":        durationMs,
				"RenderSuccess":           0,
				"RenderFailure":           1,
				"RenderValidationFailure": 0,
			}))
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}

		tileBytes, err := base64.StdEncoding.DecodeString(workerResp.BytesBase64)
		if err != nil {
			durationMs := time.Since(start).Milliseconds()
			fields := mergeLogFields(baseFields, logFields{
				"durationMs": durationMs,
				"error":      err.Error(),
			})
			logEvent("error", "distributed render failed", fields)
			emitRenderMetrics(mergeLogFields(fields, logFields{
				"RenderDurationMs":        durationMs,
				"RenderSuccess":           0,
				"RenderFailure":           1,
				"RenderValidationFailure": 0,
			}))
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}

		results = append(results, TileResult{
			Tile:  workerResp.Tile,
			Bytes: tileBytes,
		})
	}

	pixelBytes, err := assembleTileBytes(cfg, results)
	if err != nil {
		durationMs := time.Since(start).Milliseconds()
		fields := mergeLogFields(baseFields, logFields{
			"durationMs": durationMs,
			"error":      err.Error(),
		})
		logEvent("error", "distributed render failed", fields)
		emitRenderMetrics(mergeLogFields(fields, logFields{
			"RenderDurationMs":        durationMs,
			"RenderSuccess":           0,
			"RenderFailure":           1,
			"RenderValidationFailure": 0,
		}))
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	durationMs := time.Since(start).Milliseconds()
	fields := mergeLogFields(baseFields, logFields{
		"durationMs": durationMs,
		"statusCode": 200,
		"bytes":      len(pixelBytes),
	})
	logEvent("info", "distributed render succeeded", fields)
	emitRenderMetrics(mergeLogFields(fields, logFields{
		"RenderDurationMs":        durationMs,
		"RenderSuccess":           1,
		"RenderFailure":           0,
		"RenderValidationFailure": 0,
	}))

	return events.APIGatewayProxyResponse{
		StatusCode:      200,
		Headers:         map[string]string{"Content-Type": "application/octet-stream"},
		Body:            base64.StdEncoding.EncodeToString(pixelBytes),
		IsBase64Encoded: true,
	}, nil
}

func planValidatedTiles(cfg Config, tileSize int) ([]Tile, error) {
	if tileSize < minTileSize || tileSize > maxTileSize {
		return nil, fmt.Errorf("tileSize must be between %d and %d", minTileSize, maxTileSize)
	}

	tiles := planTiles(cfg, tileSize)
	if len(tiles) > maxTileCount {
		return nil, fmt.Errorf("tile count must not exceed %d", maxTileCount)
	}
	return tiles, nil
}

func workerImageFromConfig(cfg Config) WorkerImage {
	return WorkerImage{
		Width:      cfg.imgWidth,
		HeightPx:   cfg.imgHeight,
		PosX:       cfg.posX,
		PosY:       cfg.posY,
		ViewHeight: cfg.height,
		Samples:    cfg.samples,
		MaxIter:    cfg.maxIter,
	}
}

func orchestratorLogFields(cfg Config, tileSize int, tileCount int) logFields {
	return logFields{
		"mode":       renderModeOrchestrator,
		"posX":       cfg.posX,
		"posY":       cfg.posY,
		"height":     cfg.height,
		"width":      cfg.imgWidth,
		"height_px":  cfg.imgHeight,
		"pixelTotal": cfg.pixelTotal,
		"samples":    cfg.samples,
		"maxIter":    cfg.maxIter,
		"tileSize":   tileSize,
		"tileCount":  tileCount,
	}
}

func orchestratorValidationResponse(start time.Time, baseFields logFields, err error) events.APIGatewayProxyResponse {
	durationMs := time.Since(start).Milliseconds()
	fields := mergeLogFields(baseFields, logFields{
		"durationMs": durationMs,
		"statusCode": 400,
		"error":      err.Error(),
	})
	logEvent("warn", "distributed render validation failed", fields)
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
	}
}
