package main

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

type WorkerRenderRequest struct {
	RequestID string      `json:"requestId,omitempty"`
	Image     WorkerImage `json:"image"`
	Tile      Tile        `json:"tile"`
}

type WorkerImage struct {
	Width      int     `json:"width"`
	HeightPx   int     `json:"heightPx"`
	PosX       float64 `json:"posX"`
	PosY       float64 `json:"posY"`
	ViewHeight float64 `json:"viewHeight"`
	Samples    int     `json:"samples"`
	MaxIter    int     `json:"maxIter"`
}

type WorkerRenderResponse struct {
	RequestID   string `json:"requestId,omitempty"`
	Tile        Tile   `json:"tile"`
	Encoding    string `json:"encoding"`
	BytesBase64 string `json:"bytesBase64"`
	ByteLength  int    `json:"byteLength"`
}

func workerHandler(ctx context.Context, request WorkerRenderRequest) (WorkerRenderResponse, error) {
	start := time.Now()
	requestID := workerRequestID(ctx, request)
	cfg := configFromWorkerImage(request.Image)

	baseFields := mergeLogFields(workerLogFields(cfg, request.Tile), logFields{
		"requestId": requestID,
	})
	logEvent("info", "tile render request received", baseFields)

	if err := validateConfig(cfg); err != nil {
		logWorkerValidationFailure(start, baseFields, err)
		return WorkerRenderResponse{}, err
	}
	if err := validateTile(cfg, request.Tile); err != nil {
		logWorkerValidationFailure(start, baseFields, err)
		return WorkerRenderResponse{}, err
	}

	tileBytes, err := renderTileBytes(cfg, request.Tile)
	if err != nil {
		durationMs := time.Since(start).Milliseconds()
		fields := mergeLogFields(baseFields, logFields{
			"durationMs": durationMs,
			"error":      err.Error(),
		})
		logEvent("error", "tile render failed", fields)
		emitRenderMetrics(mergeLogFields(fields, logFields{
			"RenderDurationMs":        durationMs,
			"RenderSuccess":           0,
			"RenderFailure":           1,
			"RenderValidationFailure": 0,
		}))
		return WorkerRenderResponse{}, err
	}

	durationMs := time.Since(start).Milliseconds()
	fields := mergeLogFields(baseFields, logFields{
		"durationMs": durationMs,
		"bytes":      len(tileBytes),
	})
	logEvent("info", "tile render succeeded", fields)
	emitRenderMetrics(mergeLogFields(fields, logFields{
		"RenderDurationMs":        durationMs,
		"RenderSuccess":           1,
		"RenderFailure":           0,
		"RenderValidationFailure": 0,
	}))

	return WorkerRenderResponse{
		RequestID:   requestID,
		Tile:        request.Tile,
		Encoding:    "rgba",
		BytesBase64: base64.StdEncoding.EncodeToString(tileBytes),
		ByteLength:  len(tileBytes),
	}, nil
}

func configFromWorkerImage(image WorkerImage) Config {
	cfg := Config{
		posX:       image.PosX,
		posY:       image.PosY,
		height:     image.ViewHeight,
		imgWidth:   image.Width,
		imgHeight:  image.HeightPx,
		maxIter:    image.MaxIter,
		samples:    image.Samples,
		numBlocks:  1,
		numThreads: 1,
	}
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight
	if cfg.imgHeight != 0 {
		cfg.ratio = float64(cfg.imgWidth) / float64(cfg.imgHeight)
	}
	return cfg
}

func workerLogFields(cfg Config, tile Tile) logFields {
	return logFields{
		"mode":         renderModeWorker,
		"posX":         cfg.posX,
		"posY":         cfg.posY,
		"height":       cfg.height,
		"width":        cfg.imgWidth,
		"height_px":    cfg.imgHeight,
		"pixelTotal":   cfg.pixelTotal,
		"samples":      cfg.samples,
		"maxIter":      cfg.maxIter,
		"tileX":        tile.X,
		"tileY":        tile.Y,
		"tileWidth":    tile.Width,
		"tileHeightPx": tile.HeightPx,
	}
}

func logWorkerValidationFailure(start time.Time, baseFields logFields, err error) {
	durationMs := time.Since(start).Milliseconds()
	fields := mergeLogFields(baseFields, logFields{
		"durationMs": durationMs,
		"error":      err.Error(),
	})
	logEvent("warn", "tile render validation failed", fields)
	emitRenderMetrics(mergeLogFields(fields, logFields{
		"RenderDurationMs":        durationMs,
		"RenderSuccess":           0,
		"RenderFailure":           0,
		"RenderValidationFailure": 1,
	}))
}

func workerRequestID(ctx context.Context, request WorkerRenderRequest) string {
	if request.RequestID != "" {
		return request.RequestID
	}
	if lambdaCtx, ok := lambdacontext.FromContext(ctx); ok {
		return lambdaCtx.AwsRequestID
	}
	return ""
}
