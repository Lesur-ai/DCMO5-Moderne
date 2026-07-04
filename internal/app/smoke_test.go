package app

import (
	"errors"
	"testing"
)

func TestSmokeConfigFromEnv_DefaultIsDisabled(t *testing.T) {
	cfg, err := smokeConfigFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("smokeConfigFromEnv default: %v", err)
	}
	if cfg.frames != 0 || cfg.screenshot != "" {
		t.Fatalf("default smoke config = %+v, expected disabled", cfg)
	}
}

func TestSmokeConfigFromEnv_RejectsScreenshotWithoutFrames(t *testing.T) {
	_, err := smokeConfigFromEnv(func(key string) string {
		if key == envSmokeScreenshot {
			return "/tmp/dcmo5.png"
		}
		return ""
	})
	if err == nil {
		t.Fatal("screenshot without frames must be rejected, otherwise the app would never know when to capture")
	}
}

func TestSmokeConfigFromEnv_RejectsInvalidFrames(t *testing.T) {
	for _, value := range []string{"0", "-1", "abc"} {
		_, err := smokeConfigFromEnv(func(key string) string {
			if key == envSmokeFrames {
				return value
			}
			return ""
		})
		if err == nil {
			t.Fatalf("%s=%q accepted, expected validation error", envSmokeFrames, value)
		}
	}
}

func TestSmokeState_NoScreenshotQuitsAfterRenderedFrame(t *testing.T) {
	var s smokeState
	s.configure(smokeConfig{frames: 2})
	if s.shouldQuitOnUpdate() {
		t.Fatal("smoke must not quit before any rendered frame")
	}
	if s.noteRenderedFrame() {
		t.Fatal("no screenshot requested: noteRenderedFrame must not request capture")
	}
	if s.shouldQuitOnUpdate() {
		t.Fatal("smoke quit one frame too early")
	}
	s.noteRenderedFrame()
	if !s.shouldQuitOnUpdate() {
		t.Fatal("smoke did not quit after the configured rendered frame count")
	}
}

func TestSmokeState_ScreenshotWaitsForCapture(t *testing.T) {
	var s smokeState
	s.configure(smokeConfig{frames: 2, screenshot: "/tmp/dcmo5.png"})
	if s.noteRenderedFrame() {
		t.Fatal("capture requested before target rendered frame")
	}
	if s.shouldQuitOnUpdate() {
		t.Fatal("smoke must not quit before screenshot capture")
	}
	if !s.noteRenderedFrame() {
		t.Fatal("target rendered frame should request screenshot capture")
	}
	if s.shouldQuitOnUpdate() {
		t.Fatal("smoke must not quit until screenshot write is marked successful")
	}
	s.markCaptured()
	if !s.shouldQuitOnUpdate() {
		t.Fatal("smoke did not quit after screenshot capture")
	}
}

func TestSmokeState_CaptureErrorBlocksCleanQuit(t *testing.T) {
	var s smokeState
	s.configure(smokeConfig{frames: 1, screenshot: "/tmp/dcmo5.png"})
	if !s.noteRenderedFrame() {
		t.Fatal("target rendered frame should request screenshot capture")
	}
	s.markError(errors.New("disk full"))
	if s.shouldQuitOnUpdate() {
		t.Fatal("smoke must not report a clean quit after capture failure")
	}
	if s.updateError() == nil {
		t.Fatal("capture failure must be surfaced through Update")
	}
}
