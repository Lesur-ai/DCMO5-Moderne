package app

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
)

func writeSmokeScreenshot(screen *ebiten.Image, path string) error {
	if path == "" {
		return nil
	}
	bounds := screen.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return fmt.Errorf("smoke screenshot: empty screen bounds %v", bounds)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	screen.ReadPixels(img.Pix)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("smoke screenshot: create %s: %w", path, err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return fmt.Errorf("smoke screenshot: encode %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("smoke screenshot: close %s: %w", path, err)
	}
	return nil
}
