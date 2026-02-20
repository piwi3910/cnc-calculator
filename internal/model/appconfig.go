package model

// AppConfig holds application-wide preferences and default settings.
type AppConfig struct {
	// Default CNC settings applied to new projects
	DefaultKerfWidth    float64 `json:"default_kerf_width"`
	DefaultEdgeTrim     float64 `json:"default_edge_trim"`
	DefaultToolDiameter float64 `json:"default_tool_diameter"`
	DefaultFeedRate     float64 `json:"default_feed_rate"`
	DefaultPlungeRate   float64 `json:"default_plunge_rate"`
	DefaultSpindleSpeed int     `json:"default_spindle_speed"`
	DefaultSafeZ        float64 `json:"default_safe_z"`
	DefaultCutDepth     float64 `json:"default_cut_depth"`
	DefaultPassDepth    float64 `json:"default_pass_depth"`
	DefaultGCodeProfile string  `json:"default_gcode_profile"`

	// Application preferences
	AutoSaveInterval int      `json:"auto_save_interval"` // minutes, 0 = disabled
	RecentProjects   []string `json:"recent_projects"`
	Theme            string   `json:"theme"` // "light", "dark", "system"
}

// DefaultAppConfig returns an AppConfig populated with sensible defaults
// matching the values from DefaultSettings().
func DefaultAppConfig() AppConfig {
	defaults := DefaultSettings()
	return AppConfig{
		DefaultKerfWidth:    defaults.KerfWidth,
		DefaultEdgeTrim:     defaults.EdgeTrim,
		DefaultToolDiameter: defaults.ToolDiameter,
		DefaultFeedRate:     defaults.FeedRate,
		DefaultPlungeRate:   defaults.PlungeRate,
		DefaultSpindleSpeed: defaults.SpindleSpeed,
		DefaultSafeZ:        defaults.SafeZ,
		DefaultCutDepth:     defaults.CutDepth,
		DefaultPassDepth:    defaults.PassDepth,
		DefaultGCodeProfile: defaults.GCodeProfile,
		AutoSaveInterval:    0,
		RecentProjects:      []string{},
		Theme:               "system",
	}
}

// ApplyToSettings copies the default values from AppConfig into a CutSettings struct.
// This is used when creating a new project so it inherits the user's saved defaults.
func (c AppConfig) ApplyToSettings(s *CutSettings) {
	s.KerfWidth = c.DefaultKerfWidth
	s.EdgeTrim = c.DefaultEdgeTrim
	s.ToolDiameter = c.DefaultToolDiameter
	s.FeedRate = c.DefaultFeedRate
	s.PlungeRate = c.DefaultPlungeRate
	s.SpindleSpeed = c.DefaultSpindleSpeed
	s.SafeZ = c.DefaultSafeZ
	s.CutDepth = c.DefaultCutDepth
	s.PassDepth = c.DefaultPassDepth
	s.GCodeProfile = c.DefaultGCodeProfile
}
