package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func init() {
	RegisterTool("draw_image", DrawSvgDefinition, DrawSvg)
}

// DrawSvg implements the tool handler for the DrawSvg tool.

// DrawSvg renders SVG markup to a PNG image.
func DrawSvg(args map[string]any) (any, error) {
	svgCode, ok := ArgString(args, "svg")
	if !ok || svgCode == "" {
		return ErrorResult("SVG code is required"), nil
	}

	outputFile, _ := ArgString(args, "output_file")

	// Validate rsvg-convert availability
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		return ErrorResult("rsvg-convert not available. Install with: sudo apt-get install -y librsvg2-bin"), nil //nolint:nilerr
	}

	// Rasterize SVG directly using stdin (no temp file needed)
	cmd := exec.Command("rsvg-convert", "-")
	cmd.Stdin = strings.NewReader(svgCode)

	out, err := cmd.Output()
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error rasterizing SVG: %v\nMake sure the SVG code is valid.", err)), nil
	}

	resized, err := limitImageSize(out)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error processing image size: %v", err)), nil
	}

	// Optionally save the PNG to a file
	if outputFile != "" {
		if err := savePngFile(outputFile, resized); err != nil {
			return ErrorResult(err.Error()), nil
		}
	}

	// Encode and return the result
	return encodeSvgResult(resized, len(svgCode), outputFile)
}

func savePngFile(outputFile string, pngData []byte) error {
	absPath, err := ValidatePath(outputFile)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	if _, err := os.Stat(absPath); err == nil {
		return fmt.Errorf("file already exists: %s", outputFile)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking file: %w", err)
	}

	return writeFile(absPath, pngData)
}

func encodeSvgResult(pngData []byte, svgInputSize int, outputFile string) (any, error) {
	if len(pngData) > int(MaxImageSize.Load()) {
		return ErrorResult("Rasterized image too large. Maximum size is 50MB."), nil
	}

	base64Data := base64.StdEncoding.EncodeToString(pngData)

	result := map[string]any{
		"size":       len(pngData),
		"size_str":   FormatFileSize(len(pngData)),
		"is_image":   true,
		"data":       base64Data,
		"media_type": "image/png",
		"source":     "draw_svg",
		"note":       fmt.Sprintf("SVG rasterized using rsvg-convert (input: %d chars)", svgInputSize),
	}

	if outputFile != "" {
		result["saved_to"] = outputFile
	}

	return result, nil
}

// DrawSvgDefinition defines the tool schema.
var DrawSvgDefinition = map[string]any{
	"description": "Draw image as a PNG file. Takes raw SVG markup as input and saves it as base64-encoded PNG.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"svg": map[string]any{
				"type":        "string",
				"description": "Raw SVG markup code to render. Must be valid SVG content.",
			},
			"output_file": map[string]any{
				"type":        "string",
				"description": "Relative path to save the PNG image.",
			},
		},
		"required": []string{"svg", "output_file"},
	},
}
