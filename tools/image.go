package tools

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"  // registers the GIF decoder with the image package
	_ "image/jpeg" // registers the JPEG decoder with the image package
	"image/png"

	_ "golang.org/x/image/bmp" // registers the BMP decoder with the image package
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff" // registers the TIFF decoder with the image package
	_ "golang.org/x/image/webp" // registers the WebP decoder with the image package
)

// limitImageSize downscales the image to fit within MaxImageDimension x MaxImageDimension pixels.
// Returns the original data unchanged if it already fits within the limit.
func limitImageSize(imageData []byte) ([]byte, error) {
	// Decode any registered format (png, jpeg, gif, bmp, tiff, webp).
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	maxDim := int(MaxImageDimension.Load())

	// Check if downscale is needed
	if width <= maxDim && height <= maxDim {
		return imageData, nil
	}

	// Calculate target dimensions maintaining aspect ratio
	var targetW, targetH int
	if float64(width)/float64(height) > 1.0 {
		// Wider than tall - scale by width
		targetW = maxDim
		targetH = int(float64(height) * float64(maxDim) / float64(width))
	} else {
		// Taller than wide - scale by height
		targetH = maxDim
		targetW = int(float64(width) * float64(maxDim) / float64(height))
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
