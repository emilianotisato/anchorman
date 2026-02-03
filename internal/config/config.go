package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DefaultAgent  string   `toml:"default_agent"`
	ReportsOutput string   `toml:"reports_output"`
	ScanPaths     []string `toml:"scan_paths"`
}

func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		DefaultAgent:  "codex",
		ReportsOutput: filepath.Join(homeDir, "Documents", "reports"),
		ScanPaths:     []string{filepath.Join(homeDir, "Projects")},
	}
}

func AnchormanDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".anchorman"), nil
}

func ConfigPath() (string, error) {
	dir, err := AnchormanDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func DatabasePath() (string, error) {
	dir, err := AnchormanDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "db", "anchorman.sqlite"), nil
}

func ErrorLogPath() (string, error) {
	dir, err := AnchormanDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "errors.log"), nil
}

func EnsureDirectories() error {
	dir, err := AnchormanDir()
	if err != nil {
		return err
	}

	// Create main directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create db subdirectory
	dbDir := filepath.Join(dir, "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	return nil
}

func Load() (*Config, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()

	// If config file doesn't exist, create it with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := EnsureDirectories(); err != nil {
			return nil, err
		}
		if err := Save(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	// Load existing config
	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, err
	}

	// Expand ~ in paths
	cfg.ReportsOutput = expandPath(cfg.ReportsOutput)
	for i, p := range cfg.ScanPaths {
		cfg.ScanPaths[i] = expandPath(p)
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(cfg)
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

// IsPathTracked checks if a given path is under one of the configured scan paths
func (c *Config) IsPathTracked(repoPath string) bool {
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return false
	}

	for _, scanPath := range c.ScanPaths {
		absScanPath, err := filepath.Abs(scanPath)
		if err != nil {
			continue
		}

		// Check if repo path is under scan path
		rel, err := filepath.Rel(absScanPath, absRepoPath)
		if err != nil {
			continue
		}

		// If the relative path doesn't start with "..", it's under the scan path
		if len(rel) > 0 && rel[0] != '.' {
			return true
		}
		if rel == "." {
			return true
		}
	}

	return false
}
