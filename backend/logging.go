package main

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

const (
	metricNamespace  = "Mandelbrot/Renderer"
	renderModeSingle = "single"
)

type logFields map[string]any

var (
	logOutput io.Writer = os.Stdout
	logMu     sync.Mutex
)

func requestIDFromContext(ctx context.Context, request events.APIGatewayProxyRequest) string {
	if request.RequestContext.RequestID != "" {
		return request.RequestContext.RequestID
	}
	if lambdaCtx, ok := lambdacontext.FromContext(ctx); ok {
		return lambdaCtx.AwsRequestID
	}
	return ""
}

func configLogFields(cfg Config) logFields {
	return logFields{
		"mode":       renderModeSingle,
		"posX":       cfg.posX,
		"posY":       cfg.posY,
		"height":     cfg.height,
		"width":      cfg.imgWidth,
		"height_px":  cfg.imgHeight,
		"pixelTotal": cfg.pixelTotal,
		"samples":    cfg.samples,
		"maxIter":    cfg.maxIter,
		"numBlocks":  cfg.numBlocks,
		"numThreads": cfg.numThreads,
	}
}

func mergeLogFields(base logFields, extra logFields) logFields {
	fields := make(logFields, len(base)+len(extra))
	maps.Copy(fields, base)
	maps.Copy(fields, extra)
	return fields
}

func logEvent(level string, message string, fields logFields) {
	event := logFields{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"level":     level,
		"message":   message,
	}
	for key, value := range fields {
		if key == "timestamp" || key == "level" || key == "message" {
			continue
		}
		event[key] = value
	}

	writeJSONLog(event)
}

func emitRenderMetrics(fields logFields) {
	metricEvent := logFields{
		"_aws": logFields{
			"Timestamp": time.Now().UnixMilli(),
			"CloudWatchMetrics": []logFields{
				{
					"Namespace":  metricNamespace,
					"Dimensions": [][]string{{"mode"}},
					"Metrics": []logFields{
						{"Name": "RenderDurationMs", "Unit": "Milliseconds"},
						{"Name": "RenderSuccess", "Unit": "Count"},
						{"Name": "RenderFailure", "Unit": "Count"},
						{"Name": "RenderValidationFailure", "Unit": "Count"},
					},
				},
			},
		},
		"mode": renderModeSingle,
	}
	maps.Copy(metricEvent, fields)

	writeJSONLog(metricEvent)
}

func writeJSONLog(event logFields) {
	logMu.Lock()
	defer logMu.Unlock()

	_ = json.NewEncoder(logOutput).Encode(event)
}
