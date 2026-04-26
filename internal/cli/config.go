package cli

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	defaultProfileName = "default"
)

// Profile is a named prowd connection for the prow CLI.
type Profile struct {
	URL   string `mapstructure:"url"`
	Token string `mapstructure:"token"`
}

// OutputConfig controls CLI presentation defaults.
type OutputConfig struct {
	Format string `mapstructure:"format"` // table | json
	Color  string `mapstructure:"color"`  // auto | on | off (unused in Phase 0A)
}

// ConfigRoot is the prow CLI config file shape (~/.prow/config.yaml).
type ConfigRoot struct {
	DefaultProfile string             `mapstructure:"default_profile"`
	Profiles       map[string]Profile `mapstructure:"profiles"`
	Output         OutputConfig       `mapstructure:"output"`
}

// ConfigPath returns ~/.prow/config.yaml.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".prow", "config.yaml"), nil
}

// LoadConfig reads the prow CLI config from disk, creating empty defaults if missing.
func LoadConfig() (*ConfigRoot, string, error) {
	p, err := ConfigPath()
	if err != nil {
		return nil, "", err
	}

	v := viper.New()
	v.SetConfigFile(p)
	v.SetConfigType("yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, "", err
	}
	_ = v.ReadInConfig() // ignore missing file

	root := &ConfigRoot{
		DefaultProfile: firstNonEmpty(v.GetString("default_profile"), defaultProfileName),
		Profiles:       map[string]Profile{},
		Output: OutputConfig{
			Format: firstNonEmpty(v.GetString("output.format"), "table"),
			Color:  firstNonEmpty(v.GetString("output.color"), "auto"),
		},
	}

	if v.IsSet("profiles") {
		if err := v.UnmarshalKey("profiles", &root.Profiles); err != nil {
			return nil, p, err
		}
	}
	if root.Profiles == nil {
		root.Profiles = map[string]Profile{}
	}
	return root, p, nil
}

// SaveConfig writes the prow CLI config to disk (Phase 0A: tokens stored in plaintext with TODO).
func SaveConfig(path string, root *ConfigRoot) error {
	if root == nil {
		return errors.New("nil config")
	}
	if root.DefaultProfile == "" {
		root.DefaultProfile = defaultProfileName
	}
	if root.Profiles == nil {
		root.Profiles = map[string]Profile{}
	}
	if root.Output.Format == "" {
		root.Output.Format = "table"
	}
	if root.Output.Color == "" {
		root.Output.Color = "auto"
	}

	v := viper.New()
	v.Set("default_profile", root.DefaultProfile)
	v.Set("profiles", root.Profiles)
	v.Set("output", root.Output)
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	return v.WriteConfigAs(path)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
