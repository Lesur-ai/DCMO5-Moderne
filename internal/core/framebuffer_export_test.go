package core_test

// framebuffer_export_test.go — export PNG du framebuffer (tests de débogage visuel).
// Nécessite DCMO5_LONG_TESTS=1 ET la ROM MO5.

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/Lesur-ai/dcmo5/internal/spec"
)

func TestROM_Long_ExportFramebuffer_3s(t *testing.T) {
	skipIfNotLong(t)
	rom := loadROM(t)
	m, _ := newMachineWithROM(t, rom)
	m.Step(3_000_000)
	saveFramebuffer(t, m, "/tmp/dcmo5_framebuffer_3s.png")
}

func saveFramebuffer(t *testing.T, m interface {
	Framebuffer() []uint32
}, path string) {
	t.Helper()
	fb := m.Framebuffer()
	img := image.NewRGBA(image.Rect(0, 0, spec.FrameWidth, spec.FrameHeight))
	for y := 0; y < spec.FrameHeight; y++ {
		for x := 0; x < spec.FrameWidth; x++ {
			px := fb[y*spec.FrameWidth+x]
			img.Set(x, y, color.RGBA{
				R: uint8(px), G: uint8(px >> 8),
				B: uint8(px >> 16), A: uint8(px >> 24),
			})
		}
	}
	f, _ := os.Create(path)
	defer f.Close()
	png.Encode(f, img)
	t.Logf("Framebuffer → %s", path)
}
