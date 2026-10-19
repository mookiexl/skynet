package tools

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

// limitImageSize downscales the image to fit within MaxImageDimension x MaxImageDimension pixels.
// Returns the original data unchanged if it already fits within the limit.
func limitImageSize(imageData []byte) ([]byte, error) {
	// Decode PNG data
	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Check if downscale is needed
	if width <= MaxImageDimension && height <= MaxImageDimension {
		return imageData, nil
	}

	// Calculate target dimensions maintaining aspect ratio
	var targetW, targetH int
	if float64(width)/float64(height) > 1.0 {
		// Wider than tall - scale by width
		targetW = MaxImageDimension
		targetH = int(float64(height) * float64(MaxImageDimension) / float64(width))
	} else {
		// Taller than wide - scale by height
		targetH = MaxImageDimension
		targetW = int(float64(width) * float64(MaxImageDimension) / float64(height))
	}

	// Create destination image
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	// Use CatmullRom interpolation for high-quality resampling
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	// Encode back to PNG
	var outBuf bytes.Buffer
	if err := png.Encode(&outBuf, dst); err != nil {
		return nil, fmt.Errorf("failed to encode resized image: %w", err)
	}

	return outBuf.Bytes(), nil
}
