package validator

// Config controls validation behavior.
type Config struct {
	AllowedRootTags  []string
	RequireAnchorID  bool
	ExpectedAnchorID string
}

// DefaultConfig returns the default v1 component validation configuration.
func DefaultConfig() Config {
	return Config{
		AllowedRootTags: []string{"section", "header", "footer", "main", "nav", "article", "div"},
	}
}
