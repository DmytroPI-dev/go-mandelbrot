package main

import (
	"fmt"
	"math"
	"strconv"
	"sync"
)

const (
	minImageDimension = 1
	maxImageDimension = 1200
	maxPixelTotal     = 1200 * 1200
	minViewHeight     = 0.000001
	maxViewHeight     = 10.0
	maxIterations     = 5000
	maxSamples        = 64
	maxBlocks         = 1024
	maxThreads        = 64
	minTileSize       = 16
	defaultTileSize   = 128
	maxTileSize       = 512
	maxTileCount      = 144
)

type Pix struct {
	x  int
	y  int
	cr uint8
	cg uint8
	cb uint8
}

type WorkItem struct {
	initialX int
	finalX   int
	initialY int
	finalY   int
}

type Tile struct {
	X        int `json:"x"`
	Y        int `json:"y"`
	Width    int `json:"width"`
	HeightPx int `json:"heightPx"`
}

type TileResult struct {
	Tile  Tile
	Bytes []byte
}

type Config struct {
	posX       float64
	posY       float64
	height     float64
	imgWidth   int
	imgHeight  int
	pixelTotal int
	maxIter    int
	samples    int
	numBlocks  int
	numThreads int
	ratio      float64
}

// newConfigFromRequest parses the HTTP request to create a Config.
// newConfigFromRequest now accepts a map from the API Gateway event.
func newConfigFromRequest(params map[string]string) Config {
	cfg := Config{
		posX:       getFloatParam(params, "posX", -2.0),
		posY:       getFloatParam(params, "posY", -1.2),
		height:     getFloatParam(params, "height", 2.5),
		imgWidth:   getIntParam(params, "width", 1024),
		imgHeight:  getIntParam(params, "height_px", 1024),
		maxIter:    getIntParam(params, "maxIter", 1000),
		samples:    getIntParam(params, "samples", 4),
		numBlocks:  getIntParam(params, "numBlocks", 64),
		numThreads: getIntParam(params, "numThreads", 16),
	}
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight
	cfg.ratio = float64(cfg.imgWidth) / float64(cfg.imgHeight)
	return cfg
}

func validateConfig(cfg Config) error {
	if cfg.imgWidth < minImageDimension || cfg.imgWidth > maxImageDimension {
		return fmt.Errorf("width must be between %d and %d", minImageDimension, maxImageDimension)
	}
	if cfg.imgHeight < minImageDimension || cfg.imgHeight > maxImageDimension {
		return fmt.Errorf("height_px must be between %d and %d", minImageDimension, maxImageDimension)
	}
	if cfg.pixelTotal != cfg.imgWidth*cfg.imgHeight || cfg.pixelTotal > maxPixelTotal {
		return fmt.Errorf("pixel total must not exceed %d", maxPixelTotal)
	}
	if cfg.height < minViewHeight || cfg.height > maxViewHeight {
		return fmt.Errorf("height must be between %g and %g", minViewHeight, maxViewHeight)
	}
	if cfg.maxIter < 1 || cfg.maxIter > maxIterations {
		return fmt.Errorf("maxIter must be between 1 and %d", maxIterations)
	}
	if cfg.samples < 1 || cfg.samples > maxSamples {
		return fmt.Errorf("samples must be between 1 and %d", maxSamples)
	}
	if cfg.numBlocks < 1 || cfg.numBlocks > maxBlocks {
		return fmt.Errorf("numBlocks must be between 1 and %d", maxBlocks)
	}
	if cfg.numThreads < 1 || cfg.numThreads > maxThreads {
		return fmt.Errorf("numThreads must be between 1 and %d", maxThreads)
	}
	if math.IsNaN(cfg.posX) || math.IsInf(cfg.posX, 0) {
		return fmt.Errorf("posX must be a finite number")
	}
	if math.IsNaN(cfg.posY) || math.IsInf(cfg.posY, 0) {
		return fmt.Errorf("posY must be a finite number")
	}
	return nil
}

// generateFractalBytes is the main logic function, now returning a byte slice.
func generateFractalBytes(cfg Config) ([]byte, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	// Create a flat byte slice to hold RGBA values for every pixel.
	pixels := make([]byte, cfg.imgWidth*cfg.imgHeight*4)

	workBuffer := make(chan WorkItem, cfg.numBlocks)
	pixelBuffer := make(chan Pix, cfg.pixelTotal)
	var wg sync.WaitGroup

	wg.Add(cfg.numThreads)
	for i := 0; i < cfg.numThreads; i++ {
		go workerThread(&cfg, &wg, workBuffer, pixelBuffer)
	}

	go func() {
		workBufferInit(&cfg, workBuffer)
		close(workBuffer)
	}()

	go func() {
		wg.Wait()
		close(pixelBuffer)
	}()

	// Collect results and place them directly into the byte slice.
	for p := range pixelBuffer {
		// Calculate the starting index for this pixel in the flat slice.
		idx := (p.y*cfg.imgWidth + p.x) * 4
		pixels[idx] = p.cr   // R
		pixels[idx+1] = p.cg // G
		pixels[idx+2] = p.cb // B
		pixels[idx+3] = 255  // A (fully opaque)
	}

	return pixels, nil
}

func workerThread(cfg *Config, wg *sync.WaitGroup, workBuffer <-chan WorkItem, pixelBuffer chan<- Pix) {
	defer wg.Done()
	for workItem := range workBuffer {
		for x := workItem.initialX; x < workItem.finalX; x++ {
			for y := workItem.initialY; y < workItem.finalY; y++ {
				pixelBuffer <- renderPixel(*cfg, x, y)
			}
		}
	}
}

func workBufferInit(cfg *Config, workBuffer chan WorkItem) {
	for _, item := range planWorkItems(*cfg) {
		workBuffer <- item
	}
}

func planWorkItems(cfg Config) []WorkItem {
	gridSize := int(math.Ceil(math.Sqrt(float64(cfg.numBlocks))))
	if gridSize < 1 {
		gridSize = 1
	}

	items := make([]WorkItem, 0, gridSize*gridSize)
	for xBlock := 0; xBlock < gridSize; xBlock++ {
		for yBlock := 0; yBlock < gridSize; yBlock++ {
			item := WorkItem{
				initialX: xBlock * cfg.imgWidth / gridSize,
				finalX:   (xBlock + 1) * cfg.imgWidth / gridSize,
				initialY: yBlock * cfg.imgHeight / gridSize,
				finalY:   (yBlock + 1) * cfg.imgHeight / gridSize,
			}
			if item.initialX == item.finalX || item.initialY == item.finalY {
				continue
			}
			items = append(items, item)
		}
	}
	return items
}

func planTiles(cfg Config, tileSize int) []Tile {
	if tileSize < 1 {
		tileSize = 1
	}

	tiles := make([]Tile, 0, int(math.Ceil(float64(cfg.imgWidth)/float64(tileSize)))*int(math.Ceil(float64(cfg.imgHeight)/float64(tileSize))))
	for y := 0; y < cfg.imgHeight; y += tileSize {
		for x := 0; x < cfg.imgWidth; x += tileSize {
			width := min(tileSize, cfg.imgWidth-x)
			heightPx := min(tileSize, cfg.imgHeight-y)
			tiles = append(tiles, Tile{
				X:        x,
				Y:        y,
				Width:    width,
				HeightPx: heightPx,
			})
		}
	}
	return tiles
}

func renderTileBytes(cfg Config, tile Tile) ([]byte, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if err := validateTile(cfg, tile); err != nil {
		return nil, err
	}

	bytes := make([]byte, tile.Width*tile.HeightPx*4)
	for localY := 0; localY < tile.HeightPx; localY++ {
		for localX := 0; localX < tile.Width; localX++ {
			p := renderPixel(cfg, tile.X+localX, tile.Y+localY)
			idx := (localY*tile.Width + localX) * 4
			bytes[idx] = p.cr
			bytes[idx+1] = p.cg
			bytes[idx+2] = p.cb
			bytes[idx+3] = 255
		}
	}
	return bytes, nil
}

func assembleTileBytes(cfg Config, results []TileResult) ([]byte, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	bytes := make([]byte, cfg.imgWidth*cfg.imgHeight*4)
	covered := make([]bool, cfg.imgWidth*cfg.imgHeight)
	for _, result := range results {
		if err := validateTile(cfg, result.Tile); err != nil {
			return nil, err
		}
		wantLen := result.Tile.Width * result.Tile.HeightPx * 4
		if len(result.Bytes) != wantLen {
			return nil, fmt.Errorf("tile at (%d,%d) has %d bytes, want %d", result.Tile.X, result.Tile.Y, len(result.Bytes), wantLen)
		}

		for localY := 0; localY < result.Tile.HeightPx; localY++ {
			for localX := 0; localX < result.Tile.Width; localX++ {
				globalX := result.Tile.X + localX
				globalY := result.Tile.Y + localY
				pixelIdx := globalY*cfg.imgWidth + globalX
				if covered[pixelIdx] {
					return nil, fmt.Errorf("pixel (%d,%d) is covered by more than one tile", globalX, globalY)
				}
				covered[pixelIdx] = true

				sourceIdx := (localY*result.Tile.Width + localX) * 4
				targetIdx := pixelIdx * 4
				copy(bytes[targetIdx:targetIdx+4], result.Bytes[sourceIdx:sourceIdx+4])
			}
		}
	}

	for idx, isCovered := range covered {
		if !isCovered {
			return nil, fmt.Errorf("pixel (%d,%d) is not covered by any tile", idx%cfg.imgWidth, idx/cfg.imgWidth)
		}
	}

	return bytes, nil
}

func validateTile(cfg Config, tile Tile) error {
	if tile.X < 0 || tile.Y < 0 {
		return fmt.Errorf("tile origin must be non-negative")
	}
	if tile.Width < 1 || tile.HeightPx < 1 {
		return fmt.Errorf("tile dimensions must be positive")
	}
	if tile.X+tile.Width > cfg.imgWidth || tile.Y+tile.HeightPx > cfg.imgHeight {
		return fmt.Errorf("tile at (%d,%d) with size %dx%d exceeds image bounds %dx%d", tile.X, tile.Y, tile.Width, tile.HeightPx, cfg.imgWidth, cfg.imgHeight)
	}
	return nil
}

func renderPixel(cfg Config, x, y int) Pix {
	var colorR, colorG, colorB int
	for k := 0; k < cfg.samples; k++ {
		offsetX, offsetY := sampleOffsets(x, y, k)
		a := cfg.height*cfg.ratio*((float64(x)+offsetX)/float64(cfg.imgWidth)) + cfg.posX
		b := cfg.height*((float64(y)+offsetY)/float64(cfg.imgHeight)) + cfg.posY
		c := pixelColor(mandelbrotIteraction(a, b, cfg.maxIter))
		colorR += int(c.R)
		colorG += int(c.G)
		colorB += int(c.B)
	}

	return Pix{
		x:  x,
		y:  y,
		cr: uint8(float64(colorR) / float64(cfg.samples)),
		cg: uint8(float64(colorG) / float64(cfg.samples)),
		cb: uint8(float64(colorB) / float64(cfg.samples)),
	}
}

func sampleOffsets(x, y, sample int) (float64, float64) {
	if sample == 0 {
		return 0.5, 0.5
	}
	return deterministicUnitFloat(sampleSeed(x, y, sample, 0)), deterministicUnitFloat(sampleSeed(x, y, sample, 1))
}

func sampleSeed(x, y, sample, axis int) uint64 {
	seed := uint64(x+1) * 0x9e3779b185ebca87
	seed ^= uint64(y+1) * 0xc2b2ae3d27d4eb4f
	seed ^= uint64(sample+1) * 0x165667b19e3779f9
	seed ^= uint64(axis+1) * 0x85ebca77c2b2ae63
	return seed
}

func deterministicUnitFloat(seed uint64) float64 {
	seed += 0x9e3779b97f4a7c15
	seed = (seed ^ (seed >> 30)) * 0xbf58476d1ce4e5b9
	seed = (seed ^ (seed >> 27)) * 0x94d049bb133111eb
	seed ^= seed >> 31
	return float64(seed>>11) * (1.0 / (1 << 53))
}

func mandelbrotIteraction(a, b float64, maxIter int) (float64, int) {
	var x, y, xx, yy, xy float64
	for i := 0; i < maxIter; i++ {
		xx, yy, xy = x*x, y*y, x*y
		if xx+yy > 4 {
			return xx + yy, i
		}
		x = xx - yy + a
		y = 2*xy + b
	}
	return xx + yy, maxIter
}

// Param helper functions accept a map[string]string.
func getIntParam(params map[string]string, name string, defaultValue int) int {
	valStr, ok := params[name]
	if !ok {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	return val
}

func getFloatParam(params map[string]string, name string, defaultValue float64) float64 {
	valStr, ok := params[name]
	if !ok {
		return defaultValue
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultValue
	}
	return val
}
