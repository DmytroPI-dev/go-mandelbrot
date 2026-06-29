package main

import (
	"fmt"
	"log"
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

	log.Println("Finished pixel calculation.")
	return pixels, nil
}

func workerThread(cfg *Config, wg *sync.WaitGroup, workBuffer <-chan WorkItem, pixelBuffer chan<- Pix) {
	defer wg.Done()
	for workItem := range workBuffer {
		for x := workItem.initialX; x < workItem.finalX; x++ {
			for y := workItem.initialY; y < workItem.finalY; y++ {
				var colorR, colorG, colorB int
				for k := 0; k < cfg.samples; k++ {
					a := cfg.height*cfg.ratio*((float64(x)+RandFloat64())/float64(cfg.imgWidth)) + cfg.posX
					b := cfg.height*((float64(y)+RandFloat64())/float64(cfg.imgHeight)) + cfg.posY
					c := pixelColor(mandelbrotIteraction(a, b, cfg.maxIter))
					colorR += int(c.R)
					colorG += int(c.G)
					colorB += int(c.B)
				}
				pixelBuffer <- Pix{
					x, y,
					uint8(float64(colorR) / float64(cfg.samples)),
					uint8(float64(colorG) / float64(cfg.samples)),
					uint8(float64(colorB) / float64(cfg.samples)),
				}
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
