package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/http"
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

// Config holds all the parameters for generating a fractal.
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
// It provides default values for any missing parameters.
func newConfigFromRequest(r *http.Request) Config {
	cfg := Config{
		posX:       getFloatParam(r, "posX", -2.0),
		posY:       getFloatParam(r, "posY", -1.2),
		height:     getFloatParam(r, "height", 2.5),
		imgWidth:   getIntParam(r, "width", 1024),
		imgHeight:  getIntParam(r, "height_px", 1024), // Renamed to avoid conflict with 'height'
		maxIter:    getIntParam(r, "maxIter", 1000),
		samples:    getIntParam(r, "samples", 50), // Reduced default for faster web responses
		numBlocks:  getIntParam(r, "numBlocks", 64),
		numThreads: getIntParam(r, "numThreads", 16),
	}
	cfg.pixelTotal = cfg.imgWidth * cfg.imgHeight
	cfg.ratio = float64(cfg.imgWidth) / float64(cfg.imgHeight)
	return cfg
}

// GenerateMandelbrot is the entry point for our Google Cloud Function.
// It handles HTTP requests, generates the fractal, and returns it as a PNG image.
func GenerateMandelbrot(w http.ResponseWriter, r *http.Request) {
	// Parse parameters from the request URL.
	cfg := newConfigFromRequest(r)
	log.Printf("Handling request with config: %+v", cfg)

	img := image.NewRGBA(image.Rect(0, 0, cfg.imgWidth, cfg.imgHeight))

	// The concurrency pattern can be reused here.
	workBuffer := make(chan WorkItem, cfg.numBlocks)
	pixelBuffer := make(chan Pix, cfg.pixelTotal) // A large buffer is okay here for performance
	var wg sync.WaitGroup

	// Start worker goroutines
	wg.Add(cfg.numThreads)
	for i := 0; i < cfg.numThreads; i++ {
		go workerThread(&cfg, &wg, workBuffer, pixelBuffer)
	}

	// Start a goroutine to populate the work buffer and close it when done
	go func() {
		workBufferInit(&cfg, workBuffer)
		close(workBuffer)
	}()

	// Start a goroutine to wait for workers to finish, then close the pixel buffer
	go func() {
		wg.Wait()
		close(pixelBuffer)
	}()

	// Collect results from the pixel buffer until it's closed
	for p := range pixelBuffer {
		img.Set(p.x, p.y, color.RGBA{R: p.cr, G: p.cg, B: p.cb, A: 255})
	}

	log.Println("Finished generation. Encoding to PNG...")
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img)
	log.Println("Response sent.")
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
				var cr, cg, cb uint8
				cr = uint8(float64(colorR) / float64(cfg.samples))
				cg = uint8(float64(colorG) / float64(cfg.samples))
				cb = uint8(float64(colorB) / float64(cfg.samples))

				pixelBuffer <- Pix{
					x, y, cr, cg, cb,
				}

			}
		}

	}
}

func getIntParam(r *http.Request, name string, defaultValue int) int {
	valStr := r.URL.Query().Get(name)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	return val
}

func getFloatParam(r *http.Request, name string, defaultValue float64) float64 {
	valStr := r.URL.Query().Get(name)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultValue
	}
	return val
}

func mandelbrotIteraction(a, b float64, maxIter int) (float64, int) {
	var x, y, xx, yy, xy float64

	for i := 0; i < maxIter; i++ {
		xx, yy, xy = x*x, y*y, x*y
		if xx+yy > 4 {
			return xx + yy, i
		}
		// xn+1 = x^2 - y^2 + a
		x = xx - yy + a
		// yn+1 = 2xy + b
		y = 2*xy + b
	}

	return xx + yy, maxIter
}

func pixelColor(r float64, iter int) color.RGBA {
	insideSet := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	// validar se está dentro do conjunto
	// https://pt.wikipedia.org/wiki/Conjunto_de_Mandelbrot
	if r > 4 {
		// return hslToRGB(float64(0.70)-float64(iter)/3500*r, 1, 0.5)
		return hslToRGB(float64(iter)/100*r, 1, 0.5)
	}

	return insideSet
}

// main function to run a local server for testing.
// This part will not be executed when deployed as a Cloud Function.
func main() {
	http.HandleFunc("/", GenerateMandelbrot)
	log.Println("Starting local server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
