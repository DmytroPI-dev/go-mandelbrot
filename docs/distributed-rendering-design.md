# Distributed Rendering Design

## Goal

The current production path is a single Lambda that renders a complete
Mandelbrot viewport and returns raw RGBA bytes to the browser. The portfolio
path should keep that baseline, then add a distributed renderer that splits one
image request into smaller work units, renders those units in parallel, and
assembles the final image.

This should demonstrate serverless orchestration, Go worker design,
least-privilege infrastructure, observability, and cost-aware scaling.

## Proposed Architecture

```text
Browser
  -> API Gateway
  -> Render orchestrator Lambda
  -> Step Functions Map state
  -> Tile worker Lambda invocations
  -> Assembled image bytes or S3 object
  -> Browser canvas
```

The single-Lambda renderer remains available as the simple baseline. The
distributed route becomes the portfolio-grade path once it is reliable.

## Rendering Model

Use image tiles instead of only horizontal bands.

Tiles are a better fit because:

- they expose parallelism in both dimensions;
- slow, high-iteration areas are less likely to block an entire large band;
- tile counts can be capped directly for cost control;
- the same model can later support progressive rendering in the frontend.

Initial tile shape:

- default tile size: `128x128`;
- maximum rendered image size: keep current backend limits unless explicitly raised;
- maximum tile count: start conservatively, for example `64`;
- each tile returns raw RGBA bytes plus tile coordinates.

## API Shape

Keep the current render query parameters for compatibility:

- `width`
- `height_px`
- `posX`
- `posY`
- `height`
- `samples`
- `maxIter`

Add an optional mode parameter later:

- `mode=single`
- `mode=distributed`

The default can remain `single` until the distributed path is proven in AWS.

## Worker Contract

Worker input:

```json
{
  "requestId": "render request id",
  "image": {
    "width": 800,
    "heightPx": 800,
    "posX": -2.0,
    "posY": -1.25,
    "viewHeight": 2.5,
    "samples": 4,
    "maxIter": 350
  },
  "tile": {
    "x": 0,
    "y": 0,
    "width": 128,
    "heightPx": 128
  }
}
```

Worker output:

```json
{
  "requestId": "render request id",
  "tile": {
    "x": 0,
    "y": 0,
    "width": 128,
    "heightPx": 128
  },
  "encoding": "rgba",
  "bytesBase64": "..."
}
```

For the first implementation, returning tile bytes through Step Functions is
acceptable only while payloads are small. For larger images, workers should
write tile objects to S3 and return object keys to avoid Step Functions payload
limits.

## Assembly Options

### Option A: Orchestrator Assembles Bytes

The orchestrator waits for all tile outputs, copies them into a final RGBA
buffer, and returns that buffer through API Gateway.

Pros:

- simplest frontend integration;
- matches the current raw RGBA canvas flow;
- fewer moving parts.

Cons:

- constrained by Lambda response size and API Gateway payload limits;
- Step Functions payload size can become a bottleneck if tile bytes are passed
  directly.

### Option B: Tiles And Final Image In S3

Workers write tile results to S3. The orchestrator assembles a final PNG/WebP
or RGBA object in S3 and returns a signed or public CloudFront URL.

Pros:

- scales to larger renders;
- easier CDN caching;
- better for sharing portfolio renders.

Cons:

- more infrastructure and cleanup lifecycle work;
- frontend needs a slightly different loading path.

Recommendation: implement Option A first with strict limits, then move to S3
objects if we want larger images, caching, or shareable generated artifacts.

## Observability

Every render should emit structured logs with:

- `requestId`
- mode: `single` or `distributed`
- image width and height
- samples and max iterations
- tile count
- render duration
- worker duration
- failures and validation errors

Useful CloudWatch metrics:

- render duration
- tile count
- worker duration
- worker failures
- validation failures
- payload size

Step Functions execution history can become a portfolio screenshot once the
distributed path is live.

## Cost And Safety Limits

Guardrails:

- validate all input parameters before orchestration;
- cap width, height, samples, max iterations, and tile count;
- set API Gateway throttling;
- set Lambda timeouts and reserved concurrency if needed;
- set S3 lifecycle cleanup if temporary tile objects are introduced.

The distributed renderer should fail fast on invalid or excessive requests
before starting Step Functions executions.

## Implementation Steps

1. Refactor backend render logic so a tile can be rendered independently.
2. Add tests for tile coordinate mapping and deterministic tile assembly.
3. Add worker handler mode.
4. Add orchestrator handler mode.
5. Add Step Functions, worker Lambda, IAM, and log groups in Terraform.
6. Keep the current `/render` route on the single-Lambda path.
7. Add a separate distributed route or `mode=distributed` flag.
8. Add structured logs and metrics.
9. Update frontend to optionally request distributed rendering.
10. Capture architecture diagram and operational screenshots.
