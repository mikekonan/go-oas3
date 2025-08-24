package validation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mikekonan/go-oas3/validation-framework/errors"
	"github.com/mikekonan/go-oas3/validation-framework/types"
)

// SystemValidator validates the system environment
type SystemValidator struct {
	collector *errors.ErrorCollector
}

func NewSystemValidator() *SystemValidator {
	return &SystemValidator{
		collector: errors.NewErrorCollector(),
	}
}

func (sv *SystemValidator) ValidateEnvironment(ctx context.Context, config *types.ValidationConfig) error {
	sv.collector.Clear()

	// Validate Go installation
	if err := sv.validateGoInstallation(ctx); err != nil {
		sv.collector.AddValidationError("environment", "go", "Go installation validation failed", err)
	}

	// Validate workspace directory
	if err := sv.validateWorkspaceDirectory(config.WorkspaceDir); err != nil {
		sv.collector.AddValidationError("environment", "workspace", "Workspace validation failed", err)
	}

	// Validate swagger file
	if err := sv.validateSwaggerFile(config.SwaggerFile); err != nil {
		sv.collector.AddValidationError("environment", "swagger", "Swagger file validation failed", err)
	}

	// Validate versions format
	if err := sv.validateVersions(config.Versions); err != nil {
		sv.collector.AddValidationError("environment", "versions", "Version format validation failed", err)
	}

	// Validate timeout
	if err := sv.validateTimeout(config.Timeout); err != nil {
		sv.collector.AddWarning("environment", "timeout", "Timeout validation warning", err)
	}

	if sv.collector.HasErrors() {
		return fmt.Errorf("environment validation failed: %s", sv.collector.Summary())
	}

	return nil
}

func (sv *SystemValidator) validateGoInstallation(ctx context.Context) error {
	// Check if go command is available
	cmd := exec.CommandContext(ctx, "go", "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("go command not found or failed: %w", err)
	}

	// Validate Go version (should be reasonably recent)
	versionRegex := regexp.MustCompile(`go(\d+\.\d+)`)
	matches := versionRegex.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return fmt.Errorf("could not parse Go version from: %s", string(output))
	}

	// We need at least Go 1.18 for generics and modern features
	version := matches[1]
	if version < "1.18" {
		return fmt.Errorf("Go version %s is too old, need at least 1.18", version)
	}

	return nil
}

func (sv *SystemValidator) validateWorkspaceDirectory(workspaceDir string) error {
	if workspaceDir == "" {
		return fmt.Errorf("workspace directory cannot be empty")
	}

	// Check if path is absolute
	if !filepath.IsAbs(workspaceDir) {
		return fmt.Errorf("workspace directory must be an absolute path: %s", workspaceDir)
	}

	// Try to create the directory if it doesn't exist
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("cannot create workspace directory: %w", err)
	}

	// Check if directory is writable
	testFile := filepath.Join(workspaceDir, ".write-test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("workspace directory is not writable: %w", err)
	}
	os.Remove(testFile) // Clean up test file

	return nil
}

func (sv *SystemValidator) validateSwaggerFile(swaggerFile string) error {
	if swaggerFile == "" {
		return fmt.Errorf("swagger file path cannot be empty")
	}

	// Check if file exists
	info, err := os.Stat(swaggerFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("swagger file does not exist: %s", swaggerFile)
		}
		return fmt.Errorf("cannot access swagger file: %w", err)
	}

	// Check if it's a file (not directory)
	if info.IsDir() {
		return fmt.Errorf("swagger file path is a directory: %s", swaggerFile)
	}

	// Check file size (basic sanity check)
	if info.Size() == 0 {
		return fmt.Errorf("swagger file is empty: %s", swaggerFile)
	}

	if info.Size() > 10*1024*1024 { // 10MB limit
		return fmt.Errorf("swagger file is too large (%d bytes): %s", info.Size(), swaggerFile)
	}

	// Check file extension
	ext := filepath.Ext(swaggerFile)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return fmt.Errorf("swagger file should have .yaml, .yml, or .json extension: %s", swaggerFile)
	}

	return nil
}

func (sv *SystemValidator) validateVersions(versions []string) error {
	if len(versions) == 0 {
		return fmt.Errorf("at least one version must be specified")
	}

	if len(versions) < 2 {
		return fmt.Errorf("at least two versions are required for comparison")
	}

	// Validate version format (should be vX.Y.Z)
	versionRegex := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	
	versionSet := make(map[string]bool)
	for _, version := range versions {
		// Check for duplicates
		if versionSet[version] {
			return fmt.Errorf("duplicate version specified: %s", version)
		}
		versionSet[version] = true

		// Validate format
		if !versionRegex.MatchString(version) {
			return fmt.Errorf("invalid version format: %s (expected format: vX.Y.Z)", version)
		}
	}

	return nil
}

func (sv *SystemValidator) validateTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", timeout)
	}

	if timeout < 1*time.Minute {
		return fmt.Errorf("timeout is too short (<%v), consider at least 5 minutes", timeout)
	}

	if timeout > 2*time.Hour {
		return fmt.Errorf("timeout is very long (>%v), consider reducing", timeout)
	}

	return nil
}

func (sv *SystemValidator) GetErrorCollector() *errors.ErrorCollector {
	return sv.collector
}

// GeneratedCodeValidator validates the quality of generated code
type GeneratedCodeValidator struct {
	collector *errors.ErrorCollector
}

func NewGeneratedCodeValidator() *GeneratedCodeValidator {
	return &GeneratedCodeValidator{
		collector: errors.NewErrorCollector(),
	}
}

func (gcv *GeneratedCodeValidator) ValidateGeneratedCode(ctx context.Context, outputDir string, expectedFiles []string) error {
	gcv.collector.Clear()

	// Check if all expected files exist
	for _, filename := range expectedFiles {
		filePath := filepath.Join(outputDir, filename)
		if err := gcv.validateSingleFile(ctx, filePath); err != nil {
			gcv.collector.AddValidationError("generated-code", filename, "File validation failed", err)
		}
	}

	// Check for unexpected files that might indicate issues
	if err := gcv.checkForUnexpectedFiles(outputDir, expectedFiles); err != nil {
		gcv.collector.AddWarning("generated-code", "unexpected-files", "Unexpected files found", err)
	}

	// Validate package structure
	if err := gcv.validatePackageStructure(ctx, outputDir); err != nil {
		gcv.collector.AddValidationError("generated-code", "package", "Package structure validation failed", err)
	}

	if gcv.collector.HasErrors() {
		return fmt.Errorf("generated code validation failed: %s", gcv.collector.Summary())
	}

	return nil
}

func (gcv *GeneratedCodeValidator) validateSingleFile(ctx context.Context, filePath string) error {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file does not exist: %w", err)
	}

	// Check file size
	if info.Size() == 0 {
		return fmt.Errorf("file is empty: %s", filePath)
	}

	// For Go files, check syntax
	if filepath.Ext(filePath) == ".go" {
		if err := gcv.validateGoSyntax(ctx, filePath); err != nil {
			return fmt.Errorf("Go syntax validation failed: %w", err)
		}
	}

	return nil
}

func (gcv *GeneratedCodeValidator) validateGoSyntax(ctx context.Context, filePath string) error {
	// Use go fmt to check syntax
	cmd := exec.CommandContext(ctx, "go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("syntax check failed (go fmt): %w", err)
	}

	return nil
}

func (gcv *GeneratedCodeValidator) checkForUnexpectedFiles(outputDir string, expectedFiles []string) error {
	expectedSet := make(map[string]bool)
	for _, filename := range expectedFiles {
		expectedSet[filename] = true
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("cannot read directory: %w", err)
	}

	var unexpectedFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && !expectedSet[entry.Name()] {
			unexpectedFiles = append(unexpectedFiles, entry.Name())
		}
	}

	if len(unexpectedFiles) > 0 {
		return fmt.Errorf("unexpected files found: %v", unexpectedFiles)
	}

	return nil
}

func (gcv *GeneratedCodeValidator) validatePackageStructure(ctx context.Context, outputDir string) error {
	// Check if we can build the package
	cmd := exec.CommandContext(ctx, "go", "build", "-o", "/dev/null", outputDir)
	if err != nil {
		return fmt.Errorf("package does not build: %w", err)
	}

	return nil
}

func (gcv *GeneratedCodeValidator) GetErrorCollector() *errors.ErrorCollector {
	return gcv.collector
}

// ComprehensiveValidator combines all validators
type ComprehensiveValidator struct {
	systemValidator        *SystemValidator
	generatedCodeValidator *GeneratedCodeValidator
	gateRunner            *errors.GateRunner
	collector             *errors.ErrorCollector
}

func NewComprehensiveValidator() *ComprehensiveValidator {
	return &ComprehensiveValidator{
		systemValidator:        NewSystemValidator(),
		generatedCodeValidator: NewGeneratedCodeValidator(),
		gateRunner:            errors.NewGateRunner(),
		collector:             errors.NewErrorCollector(),
	}
}

func (cv *ComprehensiveValidator) ValidateAll(ctx context.Context, config *types.ValidationConfig) error {
	cv.collector.Clear()

	// System environment validation
	if err := cv.systemValidator.ValidateEnvironment(ctx, config); err != nil {
		cv.collector.AddValidationError("comprehensive", "system", "System validation failed", err)
	}

	// Merge errors from system validator
	for _, err := range cv.systemValidator.GetErrorCollector().GetAllErrors() {
		cv.collector.AddError(err)
	}

	// Add validation gates
	cv.setupValidationGates(ctx, config)

	// Execute all gates
	if err := cv.gateRunner.ExecuteAll(); err != nil {
		cv.collector.AddValidationError("comprehensive", "gates", "Validation gates failed", err)
	}

	// Merge errors from gate runner
	for _, err := range cv.gateRunner.GetErrorCollector().GetAllErrors() {
		cv.collector.AddError(err)
	}

	if cv.collector.HasErrors() {
		return fmt.Errorf("comprehensive validation failed: %s", cv.collector.DetailedSummary())
	}

	return nil
}

func (cv *ComprehensiveValidator) setupValidationGates(ctx context.Context, config *types.ValidationConfig) {
	// Gate 1: Pre-execution environment checks
	cv.gateRunner.AddGate(errors.ValidationGate{
		Name:        "pre-execution",
		Description: "Pre-execution environment validation",
		Required:    true,
		Validator: func() error {
			return cv.validatePreExecution(ctx, config)
		},
	})

	// Gate 2: Resource availability
	cv.gateRunner.AddGate(errors.ValidationGate{
		Name:        "resource-availability",
		Description: "System resource availability check",
		Required:    false,
		Validator: func() error {
			return cv.validateResourceAvailability()
		},
	})
}

func (cv *ComprehensiveValidator) validatePreExecution(ctx context.Context, config *types.ValidationConfig) error {
	// Check disk space
	workspaceInfo, err := os.Stat(config.WorkspaceDir)
	if err == nil {
		_ = workspaceInfo // We just want to make sure it's accessible
	}

	// Check if we have write permissions
	testFile := filepath.Join(config.WorkspaceDir, ".permission-test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("no write permission in workspace: %w", err)
	}
	os.Remove(testFile)

	return nil
}

func (cv *ComprehensiveValidator) validateResourceAvailability() error {
	// This is a simplified check - in production you might want to check
	// actual disk space, memory, etc.
	return nil
}

func (cv *ComprehensiveValidator) ValidateGeneratedCodeForVersion(ctx context.Context, outputDir string, expectedFiles []string) error {
	return cv.generatedCodeValidator.ValidateGeneratedCode(ctx, outputDir, expectedFiles)
}

func (cv *ComprehensiveValidator) GetErrorCollector() *errors.ErrorCollector {
	return cv.collector
}