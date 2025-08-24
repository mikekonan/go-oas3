package agents

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mikekonan/go-oas3/validation-framework/types"
)

type VersionManagerAgent struct {
	*BaseAgent
	versions      []string
	binaryPaths   map[string]string
	installedVersions map[string]*types.VersionInfo
}

func NewVersionManagerAgent(config *types.ValidationConfig) *VersionManagerAgent {
	base := NewBaseAgent("version-manager", "Version Manager Agent", config)
	return &VersionManagerAgent{
		BaseAgent:         base,
		versions:         config.Versions,
		binaryPaths:      make(map[string]string),
		installedVersions: make(map[string]*types.VersionInfo),
	}
}

func (v *VersionManagerAgent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	v.logger.Info("Starting version management for versions: %v", v.versions)

	results := make(map[string]*types.VersionInfo)
	
	for _, version := range v.versions {
		versionInfo, err := v.setupVersion(ctx, version)
		if err != nil {
			v.logger.Error("Failed to setup version %s: %v", version, err)
			versionInfo = &types.VersionInfo{
				Version:   version,
				Installed: false,
				Validated: false,
				Error:     err.Error(),
				Timestamp: time.Now(),
			}
		}
		results[version] = versionInfo
		v.installedVersions[version] = versionInfo
	}

	return results, nil
}

func (v *VersionManagerAgent) setupVersion(ctx context.Context, version string) (*types.VersionInfo, error) {
	v.logger.Info("Setting up go-oas3 version %s", version)

	versionInfo := &types.VersionInfo{
		Version:   version,
		Timestamp: time.Now(),
	}

	binaryDir := filepath.Join(v.workDir, "binaries")
	if err := v.EnsureDir(binaryDir); err != nil {
		return versionInfo, fmt.Errorf("failed to create binary directory: %w", err)
	}

	binaryName := fmt.Sprintf("go-oas3-%s", version)
	binaryPath := filepath.Join(binaryDir, binaryName)
	versionInfo.BinaryPath = binaryPath

	// Check if binary already exists and is functional
	if v.isBinaryValid(binaryPath) {
		v.logger.Info("Binary for version %s already exists and is valid", version)
		versionInfo.Installed = true
		versionInfo.Validated = true
		v.binaryPaths[version] = binaryPath
		return versionInfo, nil
	}

	// Install the specific version
	if err := v.installVersion(ctx, version, binaryPath); err != nil {
		return versionInfo, fmt.Errorf("failed to install version %s: %w", version, err)
	}

	versionInfo.Installed = true

	// Validate installation
	if err := v.validateInstallation(binaryPath); err != nil {
		return versionInfo, fmt.Errorf("installation validation failed for version %s: %w", version, err)
	}

	versionInfo.Validated = true
	v.binaryPaths[version] = binaryPath

	v.logger.Info("Successfully set up version %s at %s", version, binaryPath)
	return versionInfo, nil
}

func (v *VersionManagerAgent) installVersion(ctx context.Context, version string, binaryPath string) error {
	// Use go install to get the specific version
	moduleURL := fmt.Sprintf("github.com/mikekonan/go-oas3@%s", version)
	
	v.logger.Info("Installing %s", moduleURL)
	
	// Create a temporary directory for the installation
	tempDir, err := os.MkdirTemp("", "go-oas3-install-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory and install
	cmd := exec.CommandContext(ctx, "go", "install", moduleURL)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go install failed: %w, output: %s", err, string(output))
	}

	// Find the installed binary in GOBIN or GOPATH/bin
	goBin := os.Getenv("GOBIN")
	if goBin == "" {
		goPath := os.Getenv("GOPATH")
		if goPath == "" {
			// Default GOPATH
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			goPath = filepath.Join(home, "go")
		}
		goBin = filepath.Join(goPath, "bin")
	}

	installedBinary := filepath.Join(goBin, "go-oas3")
	
	// Copy the binary to our specific location
	if err := v.copyFile(installedBinary, binaryPath); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make sure it's executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	return nil
}

func (v *VersionManagerAgent) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}

func (v *VersionManagerAgent) isBinaryValid(binaryPath string) bool {
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return false
	}

	// Try to run --version first (for newer versions)
	cmd := exec.Command(binaryPath, "--version")
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "go-oas3") {
		return true
	}

	// Fallback: try -h for older versions like v1.0.63
	cmd = exec.Command(binaryPath, "-h")
	output, err = cmd.CombinedOutput()
	if err == nil && strings.Contains(string(output), "swagger-addr") {
		return true
	}

	return false
}

func (v *VersionManagerAgent) validateInstallation(binaryPath string) error {
	// Test basic functionality - try --version first
	cmd := exec.Command(binaryPath, "--version")
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "go-oas3") {
		return nil
	}

	// Fallback: try -h for older versions
	cmd = exec.Command(binaryPath, "-h")
	output, err = cmd.CombinedOutput()
	if err == nil && strings.Contains(string(output), "swagger-addr") {
		return nil
	}

	return fmt.Errorf("binary validation failed")
}

func (v *VersionManagerAgent) GetBinaryPath(version string) (string, error) {
	path, exists := v.binaryPaths[version]
	if !exists {
		return "", fmt.Errorf("binary path not found for version %s", version)
	}
	return path, nil
}

func (v *VersionManagerAgent) GetInstalledVersions() map[string]*types.VersionInfo {
	return v.installedVersions
}

func (v *VersionManagerAgent) Cleanup() error {
	v.logger.Info("Cleaning up version manager resources")
	// Cleanup can be implemented if needed
	return nil
}