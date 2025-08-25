package agents

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mikekonan/go-oas3/validation-framework/types"
)

type CodeGeneratorAgent struct {
	*BaseAgent
	swaggerFile     string
	versionManager  *VersionManagerAgent
	outputDirs      map[string]string
	generationResults map[string]*types.GenerationResult
	mu              sync.RWMutex
}

func NewCodeGeneratorAgent(config *types.ValidationConfig, versionManager *VersionManagerAgent) *CodeGeneratorAgent {
	base := NewBaseAgent("code-generator", "Code Generator Agent", config)
	outputDirs := make(map[string]string)
	
	// Set up output directories for each version
	for _, version := range config.Versions {
		// Clean version string for directory name (remove dots and special chars)
		cleanVersion := strings.ReplaceAll(version, ".", "_")
		outputDirs[version] = filepath.Join(config.WorkspaceDir, "generated", cleanVersion)
	}

	return &CodeGeneratorAgent{
		BaseAgent:         base,
		swaggerFile:       config.SwaggerFile,
		versionManager:    versionManager,
		outputDirs:        outputDirs,
		generationResults: make(map[string]*types.GenerationResult),
	}
}

func (c *CodeGeneratorAgent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	c.logger.Info("Starting code generation for all versions")

	if err := c.validateSwaggerFile(); err != nil {
		return nil, fmt.Errorf("swagger file validation failed: %w", err)
	}

	var wg sync.WaitGroup
	results := make(chan *types.GenerationResult, len(c.config.Versions))
	errors := make(chan error, len(c.config.Versions))

	for _, version := range c.config.Versions {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			
			result, err := c.generateForVersion(ctx, v)
			if err != nil {
				c.logger.Error("Generation failed for version %s: %v", v, err)
				errors <- fmt.Errorf("version %s: %w", v, err)
				return
			}
			
			results <- result
		}(version)
	}

	wg.Wait()
	close(results)
	close(errors)

	// Collect results
	finalResults := make(map[string]*types.GenerationResult)
	for result := range results {
		finalResults[result.Version] = result
		c.mu.Lock()
		c.generationResults[result.Version] = result
		c.mu.Unlock()
	}

	// Collect errors
	var allErrors []error
	for err := range errors {
		allErrors = append(allErrors, err)
	}

	if len(allErrors) > 0 {
		return finalResults, fmt.Errorf("generation failed for some versions: %v", allErrors)
	}

	c.logger.Info("Successfully generated code for all %d versions", len(finalResults))
	return finalResults, nil
}

func (c *CodeGeneratorAgent) generateForVersion(ctx context.Context, version string) (*types.GenerationResult, error) {
	startTime := time.Now()
	
	result := &types.GenerationResult{
		Version:   version,
		OutputDir: c.outputDirs[version],
		Metadata:  make(map[string]interface{}),
	}

	c.logger.Info("Generating code for version %s", version)

	// Get binary path from version manager
	binaryPath, err := c.versionManager.GetBinaryPath(version)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Ensure output directory exists and is clean
	if err := c.prepareOutputDirectory(result.OutputDir); err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Execute code generation
	generatedFiles, err := c.executeGeneration(ctx, binaryPath, version, result.OutputDir)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Verify generated files
	if err := c.verifyGeneratedFiles(result.OutputDir, generatedFiles); err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result, err
	}

	result.GeneratedFiles = generatedFiles
	result.Success = true
	result.Duration = time.Since(startTime)
	result.Metadata["generation_command"] = fmt.Sprintf("%s -swagger-addr %s -package validation -path %s", 
		binaryPath, c.swaggerFile, result.OutputDir)

	c.logger.Info("Successfully generated code for version %s in %v", version, result.Duration)
	return result, nil
}

func (c *CodeGeneratorAgent) validateSwaggerFile() error {
	if c.swaggerFile == "" {
		return fmt.Errorf("swagger file path is required")
	}

	// Make path absolute if it's relative
	if !filepath.IsAbs(c.swaggerFile) {
		abs, err := filepath.Abs(c.swaggerFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for swagger file: %w", err)
		}
		c.swaggerFile = abs
	}

	// Check if file exists
	if _, err := os.Stat(c.swaggerFile); os.IsNotExist(err) {
		return fmt.Errorf("swagger file does not exist: %s", c.swaggerFile)
	}

	c.logger.Info("Validated swagger file: %s", c.swaggerFile)
	return nil
}

func (c *CodeGeneratorAgent) prepareOutputDirectory(outputDir string) error {
	// Remove existing directory if it exists
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to remove existing output directory: %w", err)
	}

	// Create fresh directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	return nil
}

func (c *CodeGeneratorAgent) executeGeneration(ctx context.Context, binaryPath, version, outputDir string) ([]string, error) {
	// Create a valid Go package name from the version
	cleanVersion := strings.ReplaceAll(version, ".", "_")
	cleanVersion = strings.ReplaceAll(cleanVersion, "-", "_")
	packageName := fmt.Sprintf("validation_%s", cleanVersion[1:]) // Remove 'v' prefix
	
	// Build command arguments
	args := []string{
		"-swagger-addr", c.swaggerFile,
		"-package", packageName,
		"-path", outputDir,
	}

	c.logger.Debug("Executing: %s %s", binaryPath, strings.Join(args, " "))

	// Execute the generation command
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("code generation command failed: %w", err)
	}

	// Find generated files
	generatedFiles, err := c.findGeneratedFiles(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find generated files: %w", err)
	}

	return generatedFiles, nil
}

func (c *CodeGeneratorAgent) findGeneratedFiles(outputDir string) ([]string, error) {
	var files []string
	
	// Expected generated files based on go-oas3 pattern
	expectedFiles := []string{
		"components_gen.go",
		"routes_gen.go", 
		"spec_gen.go",
	}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(outputDir, filename)
		if _, err := os.Stat(filePath); err == nil {
			files = append(files, filename)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no generated files found in %s", outputDir)
	}

	return files, nil
}

func (c *CodeGeneratorAgent) verifyGeneratedFiles(outputDir string, files []string) error {
	for _, filename := range files {
		filePath := filepath.Join(outputDir, filename)
		
		// Check if file exists and has content
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("generated file %s not found: %w", filename, err)
		}

		if info.Size() == 0 {
			return fmt.Errorf("generated file %s is empty", filename)
		}

		// Basic syntax check - try to compile
		if strings.HasSuffix(filename, ".go") {
			if err := c.checkGoSyntax(filePath); err != nil {
				return fmt.Errorf("generated file %s has syntax errors: %w", filename, err)
			}
		}
	}

	c.logger.Debug("Verified %d generated files in %s", len(files), outputDir)
	return nil
}

func (c *CodeGeneratorAgent) checkGoSyntax(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("syntax check failed: %w, output: %s", err, string(output))
	}
	return nil
}

func (c *CodeGeneratorAgent) GetGenerationResults() map[string]*types.GenerationResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	results := make(map[string]*types.GenerationResult)
	for k, v := range c.generationResults {
		results[k] = v
	}
	return results
}

func (c *CodeGeneratorAgent) GetOutputDir(version string) (string, error) {
	dir, exists := c.outputDirs[version]
	if !exists {
		return "", fmt.Errorf("output directory not found for version %s", version)
	}
	return dir, nil
}