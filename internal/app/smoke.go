package app

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	envSmokeFrames     = "DCMO5_SMOKE_FRAMES"
	envSmokeScreenshot = "DCMO5_SMOKE_SCREENSHOT"
)

type smokeConfig struct {
	frames     int
	screenshot string
}

func smokeConfigFromEnv(getenv func(string) string) (smokeConfig, error) {
	framesRaw := strings.TrimSpace(getenv(envSmokeFrames))
	screenshot := strings.TrimSpace(getenv(envSmokeScreenshot))
	if framesRaw == "" {
		if screenshot != "" {
			return smokeConfig{}, fmt.Errorf("%s requires %s", envSmokeScreenshot, envSmokeFrames)
		}
		return smokeConfig{}, nil
	}
	frames, err := strconv.Atoi(framesRaw)
	if err != nil || frames <= 0 {
		return smokeConfig{}, fmt.Errorf("%s must be a positive integer", envSmokeFrames)
	}
	return smokeConfig{frames: frames, screenshot: screenshot}, nil
}

type smokeState struct {
	config   smokeConfig
	rendered int
	captured bool
	err      error
}

func (s *smokeState) configure(config smokeConfig) {
	*s = smokeState{config: config}
}

func (s *smokeState) enabled() bool {
	return s.config.frames > 0
}

func (s *smokeState) noteRenderedFrame() bool {
	if !s.enabled() || s.err != nil || s.captured {
		return false
	}
	s.rendered++
	return s.config.screenshot != "" && s.rendered >= s.config.frames
}

func (s *smokeState) markCaptured() {
	s.captured = true
}

func (s *smokeState) markError(err error) {
	if err != nil {
		s.err = err
	}
}

func (s *smokeState) updateError() error {
	return s.err
}

func (s *smokeState) shouldQuitOnUpdate() bool {
	if !s.enabled() || s.err != nil {
		return false
	}
	if s.config.screenshot != "" {
		return s.captured
	}
	return s.rendered >= s.config.frames
}
