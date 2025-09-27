package main
// test
import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/http"
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

const (
	posX       = -2
	posY       = -1.2
	height     = 2.5
	imgWidth   = 1024
	imgHeight  = 1024
	pixelTotal = imgWidth * imgHeight
	maxIter    = 1000
	samples    = 200
	numBlocks  = 64
	numThreads = 16
	ratio      = float64(imgWidth) / float64(imgHeight)
)

// GenerateMandelbrot is the entry point for our Google Cloud Function.
// It handles HTTP requests, generates the fractal, and returns it as a PNG image.
func GenerateMandelbrot(w http.ResponseWriter, r *http.Request) {
	// In a real-world scenario, we'd parse these from the request query parameters.
	// For now, we'll use the constants.
	log.Println("Handling request to generate Mandelbrot set...")
	img := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))

	// The concurrency pattern can be reused here.
	workBuffer := make(chan WorkItem, numBlocks)
	pixelBuffer := make(chan Pix, pixelTotal) // A large buffer is okay here for performance
	var wg sync.WaitGroup

	// Start worker goroutines
	wg.Add(numThreads)
	for i := 0; i < numThreads; i++ {
		go workerThread(&wg, workBuffer, pixelBuffer)
	}

	// Start a goroutine to populate the work buffer and close it when done
	go func() {
		workBufferInit(workBuffer)
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

func workBufferInit(workBuffer chan WorkItem) {
	var sqrt = int(math.Sqrt(numBlocks))

	for i := 0; i < sqrt; i++ {
		for j := 0; j < sqrt; j++ {
			workBuffer <- WorkItem{
				initialX: i * (imgWidth / sqrt),
				finalX:   (i + 1) * (imgWidth / sqrt),
				initialY: j * (imgHeight / sqrt),
				finalY:   (j + 1) * (imgHeight / sqrt),
			}
		}
	}
}

func workerThread(wg *sync.WaitGroup, workBuffer <-chan WorkItem, pixelBuffer chan<- Pix) {
	defer wg.Done()

	for workItem := range workBuffer {
		for x := workItem.initialX; x < workItem.finalX; x++ {
			for y := workItem.initialY; y < workItem.finalY; y++ {
				var colorR, colorG, colorB int
				for k := 0; k < samples; k++ {
					a := height*ratio*((float64(x)+RandFloat64())/float64(imgWidth)) + posX
					b := height*((float64(y)+RandFloat64())/float64(imgHeight)) + posY
					c := pixelColor(mandelbrotIteraction(a, b, maxIter))
					colorR += int(c.R)
					colorG += int(c.G)
					colorB += int(c.B)
				}
				var cr, cg, cb uint8
				cr = uint8(float64(colorR) / float64(samples))
				cg = uint8(float64(colorG) / float64(samples))
				cb = uint8(float64(colorB) / float64(samples))

				pixelBuffer <- Pix{
					x, y, cr, cg, cb,
				}

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
