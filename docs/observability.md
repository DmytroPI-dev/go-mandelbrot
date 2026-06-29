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

## Viewing Metrics

CloudWatch Console:

1. Open CloudWatch.
2. Go to Metrics, All metrics.
3. Choose the `Mandelbrot/Renderer` namespace.
4. Choose the `mode` dimension.
5. Select `RenderSuccess`, `RenderFailure`, `RenderValidationFailure`, or
   `RenderDurationMs` with `mode=single`.

Useful CLI checks:

```sh
aws cloudwatch list-metrics \
  --namespace Mandelbrot/Renderer \
  --region eu-central-1 \
  --profile default
```

```sh
aws cloudwatch get-metric-statistics \
  --namespace Mandelbrot/Renderer \
  --metric-name RenderFailure \
  --dimensions Name=mode,Value=single \
  --statistics Sum \
  --period 60 \
  --start-time "$(date -u -d '30 minutes ago' +%Y-%m-%dT%H:%M:%SZ)" \
  --end-time "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --region eu-central-1 \
  --profile default
```

`RenderValidationFailure` means the request was rejected by input validation.
`RenderFailure` means a validated render failed internally. It is expected to be
zero during normal operation.

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

Metric events:

```sql
fields @timestamp, mode, RenderSuccess, RenderFailure, RenderValidationFailure, RenderDurationMs
| filter ispresent(_aws)
| sort @timestamp desc
| limit 20
```
