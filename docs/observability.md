# Observability

## Current Approach

The backend emits newline-delimited JSON to stdout. AWS Lambda forwards stdout
to CloudWatch Logs.

The handler emits:

- `render request received`
- `render validation failed`
- `render failed`
- `render succeeded`

Each terminal render event also emits an AWS Embedded Metric Format record.
CloudWatch can extract metrics from these log events without a sidecar, agent,
Prometheus server, or Grafana deployment.

## Log Fields

Common fields:

- `timestamp`
- `level`
- `message`
- `requestId`
- `mode`
- `width`
- `height_px`
- `pixelTotal`
- `samples`
- `maxIter`
- `numBlocks`
- `numThreads`
- `statusCode`
- `durationMs`

Error fields:

- `error`

Success fields:

- `bytes`

## CloudWatch Metrics

Metric namespace:

```text
Mandelbrot/Renderer
```

Dimension:

```text
mode=single
```

Metrics:

- `RenderDurationMs`
- `RenderSuccess`
- `RenderFailure`
- `RenderValidationFailure`

This keeps custom metric cardinality low. The distributed renderer can later
reuse the same namespace with `mode=distributed`.

## Example CloudWatch Logs Insights Queries

Recent validation failures:

```sql
fields @timestamp, requestId, error, width, height_px, samples, maxIter
| filter message = "render validation failed"
| sort @timestamp desc
| limit 20
```

Slow successful renders:

```sql
fields @timestamp, requestId, durationMs, width, height_px, samples, maxIter, bytes
| filter message = "render succeeded"
| sort durationMs desc
| limit 20
```

Error summary:

```sql
fields @timestamp, requestId, message, error
| filter level in ["warn", "error"]
| sort @timestamp desc
| limit 50
```
