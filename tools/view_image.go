package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	RegisterTool("view_image", ViewImageDefinition, ViewImage)
}

var imageMIMETypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".png": "image/png", ".gif": "image/gif",
	".webp": "image/webp", ".bmp": "image/bmp",
	".tiff": "image/tiff", ".tif": "image/tiff",
	".svg": "image/svg+xml",
}

//
// ViewImage implements the tool handler for the ViewImage tool.

// ViewImage reads an image and returns base64-encoded data.
func ViewImage(args map[string]any) (any, error) {
	path, ok := ArgString(args, "path")
	if !ok || path == "" {
		return ErrorResult("Image path is required"), nil
	}

	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	mimeType, isImage := imageMIMETypes[ext]
	if !isImage {
		supported := make([]string, 0, len(imageMIMETypes))
		for k := range imageMIMETypes {
			supported = append(supported, k)
		}
		return ErrorResult(fmt.Sprintf("Unsupported image format '%s'. Supported formats: %s", ext, strings.Join(supported, ", "))), nil
	}

	if ext == ".svg" {
		return rasterizeSVG(absPath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error reading image: %v", err)), nil
	}

	resized, err := limitImageSize(data)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error processing image size: %v", err)), nil
	}

	return encodeAndReturn(mimeType, resized)
}

func encodeAndReturn(mimeType string, data []byte) (any, error) {
	if len(data) > int(MaxImageSize.Load()) {
		return ErrorResult("Image too large. Maximum size is 50MB."), nil
	}

	base64Data := base64.StdEncoding.EncodeToString(data)

	return map[string]any{
		"size": len(data), "size_str": FormatFileSize(len(data)), "data": base64Data,
		"is_image": true, "media_type": mimeType,
	}, nil
}

func rasterizeSVG(svgPath string) (any, error) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		return ErrorResult("rsvg-convert not available"), nil //nolint:nilerr
	}

	cmd := exec.Command("rsvg-convert", svgPath)
	out, err := cmd.Output()
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error rasterizing SVG: %v", err)), nil
	}

	resized, err := limitImageSize(out)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error processing image size: %v", err)), nil
	}

	result, err := encodeAndReturn("image/png", resized)
	if err != nil {
		return result, err
	}
	if m, ok := result.(map[string]any); ok {
		m["note"] = "SVG rasterized using rsvg-convert"
	}
	return result, nil
}

// ViewImageDefinition defines the tool schema.
var ViewImageDefinition = map[string]any{
	"description": "Reads an image file and returns its content as base64-encoded data with MIME type, suitable for Vision-Language Model (VLM) input. Supports JPEG, PNG, GIF, WebP, BMP, TIFF, and SVG formats. SVG files are automatically rasterized to PNG for VLM compatibility.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Image file path relative to current directory",
			},
		},
		"required": []string{"path"},
	},
}
