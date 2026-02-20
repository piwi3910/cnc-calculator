package model

import "testing"

func TestDefaultAppConfigMatchesDefaultSettings(t *testing.T) {
	cfg := DefaultAppConfig()
	defaults := DefaultSettings()

	if cfg.DefaultKerfWidth != defaults.KerfWidth {
		t.Errorf("KerfWidth mismatch: config=%f settings=%f", cfg.DefaultKerfWidth, defaults.KerfWidth)
	}
	if cfg.DefaultToolDiameter != defaults.ToolDiameter {
		t.Errorf("ToolDiameter mismatch: config=%f settings=%f", cfg.DefaultToolDiameter, defaults.ToolDiameter)
	}
	if cfg.DefaultFeedRate != defaults.FeedRate {
		t.Errorf("FeedRate mismatch: config=%f settings=%f", cfg.DefaultFeedRate, defaults.FeedRate)
	}
	if cfg.DefaultGCodeProfile != defaults.GCodeProfile {
		t.Errorf("GCodeProfile mismatch: config=%s settings=%s", cfg.DefaultGCodeProfile, defaults.GCodeProfile)
	}
	if cfg.Theme != "system" {
		t.Errorf("expected default theme=system, got %s", cfg.Theme)
	}
	if cfg.RecentProjects == nil {
		t.Error("RecentProjects should not be nil")
	}
}

func TestApplyToSettings(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.DefaultKerfWidth = 5.0
	cfg.DefaultFeedRate = 3000.0
	cfg.DefaultGCodeProfile = "Grbl"

	s := DefaultSettings()
	cfg.ApplyToSettings(&s)

	if s.KerfWidth != 5.0 {
		t.Errorf("expected KerfWidth=5.0, got %f", s.KerfWidth)
	}
	if s.FeedRate != 3000.0 {
		t.Errorf("expected FeedRate=3000.0, got %f", s.FeedRate)
	}
	if s.GCodeProfile != "Grbl" {
		t.Errorf("expected GCodeProfile=Grbl, got %s", s.GCodeProfile)
	}
}
