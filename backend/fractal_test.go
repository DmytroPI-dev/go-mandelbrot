package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func validTestConfig() Config {
	cfg := newConfigFromRequest(map[string]string{
		"width":      "8",
		"height_px":  "6",
		"height":     "2.5",
		"samples":    "1",
		"maxIter":    "20",
		"numBlocks":  "4",
		"numThreads": "1",
	})
	return cfg
}

func TestNewConfigFromRequestDefaults(t *testing.T) {
	cfg := newConfigFromRequest(nil)

	if cfg.posX != -2.0 {
		t.Fatalf("posX = %v, want -2.0", cfg.posX)
	}
	if cfg.posY != -1.2 {
		t.Fatalf("posY = %v, want -1.2", cfg.posY)
	}
	if cfg.height != 2.5 {
		t.Fatalf("height = %v, want 2.5", cfg.height)
	}
	if cfg.imgWidth != 1024 || cfg.imgHeight != 1024 {
		t.Fatalf("image size = %dx%d, want 1024x1024", cfg.imgWidth, cfg.imgHeight)
	}
	if cfg.maxIter != 1000 {
		t.Fatalf("maxIter = %d, want 1000", cfg.maxIter)
	}
	if cfg.samples != 4 {
		t.Fatalf("samples = %d, want 4", cfg.samples)
	}
	if cfg.numBlocks != 64 {
		t.Fatalf("numBlocks = %d, want 64", cfg.numBlocks)
	}
	if cfg.numThreads != 16 {
		t.Fatalf("numThreads = %d, want 16", cfg.numThreads)
	}
	if cfg.pixelTotal != 1024*1024 {
		t.Fatalf("pixelTotal = %d, want %d", cfg.pixelTotal, 1024*1024)
	}
	if cfg.ratio != 1 {
		t.Fatalf("ratio = %v, want 1", cfg.ratio)
	}
}

func TestNewConfigFromRequestParsesValues(t *testing.T) {
	cfg := newConfigFromRequest(map[string]string{
		"posX":       "-0.75",
		"posY":       "0.1",
		"height":     "1.5",
		"width":      "320",
		"height_px":  "160",
		"samples":    "3",
		"maxIter":    "250",
		"numBlocks":  "9",
		"numThreads": "2",
	})

	if cfg.posX != -0.75 || cfg.posY != 0.1 || cfg.height != 1.5 {
		t.Fatalf("viewport = (%v, %v, %v), want (-0.75, 0.1, 1.5)", cfg.posX, cfg.posY, cfg.height)
	}
	if cfg.imgWidth != 320 || cfg.imgHeight != 160 {
		t.Fatalf("image size = %dx%d, want 320x160", cfg.imgWidth, cfg.imgHeight)
	}
	if cfg.samples != 3 || cfg.maxIter != 250 || cfg.numBlocks != 9 || cfg.numThreads != 2 {
		t.Fatalf("render controls = samples:%d maxIter:%d blocks:%d threads:%d", cfg.samples, cfg.maxIter, cfg.numBlocks, cfg.numThreads)
	}
	if cfg.pixelTotal != 320*160 {
		t.Fatalf("pixelTotal = %d, want %d", cfg.pixelTotal, 320*160)
	}
	if cfg.ratio != 2 {
		t.Fatalf("ratio = %v, want 2", cfg.ratio)
	}
}

func TestValidateConfigRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "width too large",
			mutate: func(cfg *Config) {
				cfg.imgWidth = maxImageDimension + 1
				cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight
			},
			wantErr: "width",
		},
		{
			name: "height_px too small",
			mutate: func(cfg *Config) {
				cfg.imgHeight = 0
				cfg.pixelTotal = 0
			},
			wantErr: "height_px",
		},
		{
			name: "view height too small",
			mutate: func(cfg *Config) {
				cfg.height = 0
			},
			wantErr: "height",
		},
		{
			name: "maxIter too large",
			mutate: func(cfg *Config) {
				cfg.maxIter = maxIterations + 1
			},
			wantErr: "maxIter",
		},
		{
			name: "samples too large",
			mutate: func(cfg *Config) {
				cfg.samples = maxSamples + 1
			},
			wantErr: "samples",
		},
		{
			name: "numBlocks too small",
			mutate: func(cfg *Config) {
				cfg.numBlocks = 0
			},
			wantErr: "numBlocks",
		},
		{
			name: "numThreads too large",
			mutate: func(cfg *Config) {
				cfg.numThreads = maxThreads + 1
			},
			wantErr: "numThreads",
		},
		{
			name: "non-finite posX",
			mutate: func(cfg *Config) {
				*cfg = newConfigFromRequest(map[string]string{"posX": "NaN"})
			},
			wantErr: "posX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validTestConfig()
			tt.mutate(&cfg)

			err := validateConfig(cfg)
			if err == nil {
				t.Fatal("validateConfig returned nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPlanWorkItemsCoversEveryPixelOnce(t *testing.T) {
	cfg := validTestConfig()
	cfg.imgWidth = 7
	cfg.imgHeight = 5
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight
	cfg.numBlocks = 6

	items := planWorkItems(cfg)
	covered := make([][]int, cfg.imgHeight)
	for y := range covered {
		covered[y] = make([]int, cfg.imgWidth)
	}

	for _, item := range items {
		if item.initialX < 0 || item.finalX > cfg.imgWidth || item.initialY < 0 || item.finalY > cfg.imgHeight {
			t.Fatalf("work item out of bounds: %+v", item)
		}
		if item.initialX >= item.finalX || item.initialY >= item.finalY {
			t.Fatalf("empty work item: %+v", item)
		}
		for x := item.initialX; x < item.finalX; x++ {
			for y := item.initialY; y < item.finalY; y++ {
				covered[y][x]++
			}
		}
	}

	for y := 0; y < cfg.imgHeight; y++ {
		for x := 0; x < cfg.imgWidth; x++ {
			if covered[y][x] != 1 {
				t.Fatalf("pixel (%d,%d) covered %d times, want 1", x, y, covered[y][x])
			}
		}
	}
}

func TestPlanTilesCoversEveryPixelOnce(t *testing.T) {
	cfg := validTestConfig()
	cfg.imgWidth = 7
	cfg.imgHeight = 5
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight

	tiles := planTiles(cfg, 3)
	covered := make([][]int, cfg.imgHeight)
	for y := range covered {
		covered[y] = make([]int, cfg.imgWidth)
	}

	for _, tile := range tiles {
		if err := validateTile(cfg, tile); err != nil {
			t.Fatalf("validateTile(%+v) returned error: %v", tile, err)
		}
		for x := tile.X; x < tile.X+tile.Width; x++ {
			for y := tile.Y; y < tile.Y+tile.HeightPx; y++ {
				covered[y][x]++
			}
		}
	}

	for y := 0; y < cfg.imgHeight; y++ {
		for x := 0; x < cfg.imgWidth; x++ {
			if covered[y][x] != 1 {
				t.Fatalf("pixel (%d,%d) covered %d times, want 1", x, y, covered[y][x])
			}
		}
	}
}

func TestRenderTileBytesReturnsTileRGBABytes(t *testing.T) {
	cfg := validTestConfig()
	tile := Tile{X: 1, Y: 2, Width: 3, HeightPx: 2}

	got, err := renderTileBytes(cfg, tile)
	if err != nil {
		t.Fatalf("renderTileBytes returned error: %v", err)
	}
	if len(got) != tile.Width*tile.HeightPx*4 {
		t.Fatalf("len(bytes) = %d, want %d", len(got), tile.Width*tile.HeightPx*4)
	}
	for i := 3; i < len(got); i += 4 {
		if got[i] != 255 {
			t.Fatalf("alpha byte at %d = %d, want 255", i, got[i])
		}
	}
}

func TestAssembledTilesMatchFullRender(t *testing.T) {
	cfg := validTestConfig()
	cfg.imgWidth = 9
	cfg.imgHeight = 7
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight
	cfg.samples = 3
	cfg.numBlocks = 5
	cfg.numThreads = 2

	full, err := generateFractalBytes(cfg)
	if err != nil {
		t.Fatalf("generateFractalBytes returned error: %v", err)
	}

	tiles := planTiles(cfg, 4)
	results := make([]TileResult, 0, len(tiles))
	for _, tile := range tiles {
		tileBytes, err := renderTileBytes(cfg, tile)
		if err != nil {
			t.Fatalf("renderTileBytes(%+v) returned error: %v", tile, err)
		}
		results = append(results, TileResult{
			Tile:  tile,
			Bytes: tileBytes,
		})
	}

	assembled, err := assembleTileBytes(cfg, results)
	if err != nil {
		t.Fatalf("assembleTileBytes returned error: %v", err)
	}
	if !bytes.Equal(assembled, full) {
		t.Fatal("assembled tile bytes do not match full render bytes")
	}
}

func TestAssembleTileBytesRejectsMissingCoverage(t *testing.T) {
	cfg := validTestConfig()
	cfg.imgWidth = 4
	cfg.imgHeight = 4
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight

	tile := Tile{X: 0, Y: 0, Width: 2, HeightPx: 2}
	tileBytes, err := renderTileBytes(cfg, tile)
	if err != nil {
		t.Fatalf("renderTileBytes returned error: %v", err)
	}

	_, err = assembleTileBytes(cfg, []TileResult{{Tile: tile, Bytes: tileBytes}})
	if err == nil {
		t.Fatal("assembleTileBytes returned nil error, want missing coverage error")
	}
	if !strings.Contains(err.Error(), "not covered") {
		t.Fatalf("error = %q, want missing coverage message", err.Error())
	}
}

func TestGenerateFractalBytesReturnsRGBABytes(t *testing.T) {
	cfg := validTestConfig()
	cfg.imgWidth = 4
	cfg.imgHeight = 3
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight

	got, err := generateFractalBytes(cfg)
	if err != nil {
		t.Fatalf("generateFractalBytes returned error: %v", err)
	}
	if len(got) != cfg.pixelTotal*4 {
		t.Fatalf("len(bytes) = %d, want %d", len(got), cfg.pixelTotal*4)
	}
	for i := 3; i < len(got); i += 4 {
		if got[i] != 255 {
			t.Fatalf("alpha byte at %d = %d, want 255", i, got[i])
		}
	}
}

func TestHandlerRejectsInvalidRequest(t *testing.T) {
	resp, err := handler(context.Background(), events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{"width": "0"},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, "width") {
		t.Fatalf("body = %q, want validation message mentioning width", resp.Body)
	}
}

func TestHandlerReturnsBase64EncodedRGBABytes(t *testing.T) {
	resp, err := handler(context.Background(), events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{
			"width":      "2",
			"height_px":  "2",
			"samples":    "1",
			"maxIter":    "10",
			"numBlocks":  "1",
			"numThreads": "1",
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
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

	decoded, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	if len(decoded) != 2*2*4 {
		t.Fatalf("decoded length = %d, want %d", len(decoded), 2*2*4)
	}
}
