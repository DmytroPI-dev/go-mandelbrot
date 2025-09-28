package main

import (
	"log"
	"math"
	"strconv"
	"sync"
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
		samples:    getIntParam(params, "samples", 50),
		numBlocks:  getIntParam(params, "numBlocks", 64),
		numThreads: getIntParam(params, "numThreads", 16),
	}
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight
	cfg.ratio = float64(cfg.imgWidth) / float64(cfg.imgHeight)
	return cfg
}

// generateFractalBytes is the main logic function, now returning a byte slice.
func generateFractalBytes(cfg Config) ([]byte, error) {
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
	var sqrt = int(math.Sqrt(float64(cfg.numBlocks)))
	for i := 0; i < sqrt; i++ {
		for j := 0; j < sqrt; j++ {
			workBuffer <- WorkItem{
				initialX: i * (cfg.imgWidth / sqrt),
				finalX:   (i + 1) * (cfg.imgWidth / sqrt),
				initialY: j * (cfg.imgHeight / sqrt),
				finalY:   (j + 1) * (cfg.imgHeight / sqrt),
			}
		}
	}
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
