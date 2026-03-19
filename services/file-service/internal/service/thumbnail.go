package service

import (
	"bytes"
	"fmt"
	"os/exec"
)

// resizeImage uses ImageMagick's convert tool to resize raw image bytes.
// Falls back gracefully when ImageMagick is not installed.
func resizeImage(data []byte, width, height int, contentType string) ([]byte, error) {
	convert, err := exec.LookPath("convert")
	if err != nil {
		return nil, fmt.Errorf("imagemagick not found: %w", err)
	}

	// Read from stdin, resize to fit within WxH, write JPEG to stdout.
	geometry := fmt.Sprintf("%dx%d>", width, height)
	cmd := exec.Command(convert,
		"-",              // stdin
		"-resize", geometry,
		"-quality", "85",
		"jpeg:-",         // stdout as JPEG
	)
	cmd.Path = convert
	cmd.Stdin = bytes.NewReader(data)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("imagemagick convert: %w", err)
	}
	return out, nil
}
