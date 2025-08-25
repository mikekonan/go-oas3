package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mikekonan/go-oas3/validation-framework/types"
)

type ConfigLoader struct{}

func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{}
}

func (c *ConfigLoader) LoadDefault() (*types.ValidationConfig, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Default configuration based on PRP specifications
	config := &types.ValidationConfig{
		Versions:      []string{"v1.0.63", "v1.0.65", "v1.0.66"},
		SwaggerFile:   filepath.Join(cwd, "example", "swagger.yaml"),
		WorkspaceDir:  filepath.Join(cwd, "validation-workspace"),
		OutputFormats: []string{"markdown", "json"},
		Parallel:      true,
		Timeout:       30 * time.Minute,
		BinaryPaths:   make(map[string]string),
	}

	return config, nil
}

func (c *ConfigLoader) LoadFromArgs(args []string) (*types.ValidationConfig, error) {
	config, err := c.LoadDefault()
	if err != nil {
		return nil, err
	}

	// Simple argument parsing - in production this would use a proper flag library
	for i, arg := range args {
		switch arg {
		case "--versions":
			if i+1 < len(args) {
				// Parse comma-separated versions
				versions := parseVersions(args[i+1])
				config.Versions = versions
			}
		case "--swagger-file":
			if i+1 < len(args) {
				config.SwaggerFile = args[i+1]
			}
		case "--workspace":
			if i+1 < len(args) {
				config.WorkspaceDir = args[i+1]
			}
		case "--formats":
			if i+1 < len(args) {
				config.OutputFormats = parseFormats(args[i+1])
			}
		case "--timeout":
			if i+1 < len(args) {
				if duration, err := time.ParseDuration(args[i+1]); err == nil {
					config.Timeout = duration
				}
			}
		case "--sequential":
			config.Parallel = false
		}
	}

	return config, nil
}

func parseVersions(versionsStr string) []string {
	// Simple comma-separated parsing
	if versionsStr == "" {
		return []string{"v1.0.63", "v1.0.65", "v1.0.66"}
	}
	
	// Split by comma and clean up
	var versions []string
	parts := splitAndTrim(versionsStr, ",")
	for _, part := range parts {
		if part != "" {
			// Ensure version has 'v' prefix
			if part[0] != 'v' {
				part = "v" + part
			}
			versions = append(versions, part)
		}
	}
	
	return versions
}

func parseFormats(formatsStr string) []string {
	if formatsStr == "" {
		return []string{"markdown", "json"}
	}
	
	return splitAndTrim(formatsStr, ",")
}

func splitAndTrim(s, sep string) []string {
	var result []string
	parts := split(s, sep)
	for _, part := range parts {
		trimmed := trim(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func split(s, sep string) []string {
	// Simple string splitting
	var result []string
	start := 0
	
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	
	return result
}

func trim(s string) string {
	// Simple whitespace trimming
	start := 0
	end := len(s)
	
	for start < end && isWhitespace(s[start]) {
		start++
	}
	
	for end > start && isWhitespace(s[end-1]) {
		end--
	}
	
	return s[start:end]
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func (c *ConfigLoader) ValidateConfig(config *types.ValidationConfig) error {
	if len(config.Versions) < 2 {
		return fmt.Errorf("at least 2 versions are required for comparison")
	}

	if config.SwaggerFile == "" {
		return fmt.Errorf("swagger file path is required")
	}

	// Check if swagger file exists
	if _, err := os.Stat(config.SwaggerFile); os.IsNotExist(err) {
		return fmt.Errorf("swagger file does not exist: %s", config.SwaggerFile)
	}

	if config.WorkspaceDir == "" {
		return fmt.Errorf("workspace directory is required")
	}

	// Ensure workspace directory is absolute
	if !filepath.IsAbs(config.WorkspaceDir) {
		abs, err := filepath.Abs(config.WorkspaceDir)
		if err != nil {
			return fmt.Errorf("failed to resolve workspace directory: %w", err)
		}
		config.WorkspaceDir = abs
	}

	// Ensure swagger file is absolute
	if !filepath.IsAbs(config.SwaggerFile) {
		abs, err := filepath.Abs(config.SwaggerFile)
		if err != nil {
			return fmt.Errorf("failed to resolve swagger file path: %w", err)
		}
		config.SwaggerFile = abs
	}

	return nil
}

func (c *ConfigLoader) PrintConfig(config *types.ValidationConfig) {
	fmt.Println("Validation Framework Configuration:")
	fmt.Printf("  Versions: %v\n", config.Versions)
	fmt.Printf("  Swagger File: %s\n", config.SwaggerFile)
	fmt.Printf("  Workspace Directory: %s\n", config.WorkspaceDir)
	fmt.Printf("  Output Formats: %v\n", config.OutputFormats)
	fmt.Printf("  Parallel Execution: %t\n", config.Parallel)
	fmt.Printf("  Timeout: %v\n", config.Timeout)
}