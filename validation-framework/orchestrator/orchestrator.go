package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mikekonan/go-oas3/validation-framework/agents"
	"github.com/mikekonan/go-oas3/validation-framework/types"
)

type ValidationOrchestrator struct {
	config           *types.ValidationConfig
	versionManager   *agents.VersionManagerAgent
	codeGenerator    *agents.CodeGeneratorAgent
	analysisAgent    *agents.AnalysisAgent
	reportingAgent   *agents.ReportingAgent
	executionContext *types.ExecutionContext
	logger           types.Logger
}

func NewValidationOrchestrator(config *types.ValidationConfig) (*ValidationOrchestrator, error) {
	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create execution context
	execContext := &types.ExecutionContext{
		Config:    config,
		StartTime: time.Now(),
		Results:   make(map[string]interface{}),
		Errors:    make([]error, 0),
		Logger:    &agents.DefaultLogger{},
	}

	// Create agents
	versionManager := agents.NewVersionManagerAgent(config)
	codeGenerator := agents.NewCodeGeneratorAgent(config, versionManager)
	analysisAgent := agents.NewAnalysisAgent(config, codeGenerator)
	reportingAgent := agents.NewReportingAgent(config, analysisAgent, codeGenerator, versionManager)

	// Set up logger for all agents
	logger := &agents.DefaultLogger{}
	versionManager.SetLogger(logger)
	codeGenerator.SetLogger(logger)
	analysisAgent.SetLogger(logger)
	reportingAgent.SetLogger(logger)

	return &ValidationOrchestrator{
		config:           config,
		versionManager:   versionManager,
		codeGenerator:    codeGenerator,
		analysisAgent:    analysisAgent,
		reportingAgent:   reportingAgent,
		executionContext: execContext,
		logger:           logger,
	}, nil
}

func validateConfig(config *types.ValidationConfig) error {
	if config == nil {
		return fmt.Errorf("configuration is required")
	}

	if len(config.Versions) == 0 {
		return fmt.Errorf("at least one version must be specified")
	}

	if config.SwaggerFile == "" {
		return fmt.Errorf("swagger file path is required")
	}

	if config.WorkspaceDir == "" {
		return fmt.Errorf("workspace directory is required")
	}

	// Validate that we have at least 2 versions for comparison
	if len(config.Versions) < 2 {
		return fmt.Errorf("at least 2 versions are required for comparison")
	}

	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Minute
	}

	return nil
}

func (o *ValidationOrchestrator) Execute(ctx context.Context) error {
	o.logger.Info("Starting multi-agent cross-version validation")
	o.logger.Info("Versions: %v", o.config.Versions)
	o.logger.Info("Swagger file: %s", o.config.SwaggerFile)
	o.logger.Info("Workspace: %s", o.config.WorkspaceDir)

	// Set timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, o.config.Timeout)
	defer cancel()

	// Execute validation pipeline
	if err := o.executeValidationPipeline(timeoutCtx); err != nil {
		return fmt.Errorf("validation pipeline failed: %w", err)
	}

	duration := time.Since(o.executionContext.StartTime)
	o.logger.Info("Validation completed successfully in %v", duration)
	
	return nil
}

func (o *ValidationOrchestrator) executeValidationPipeline(ctx context.Context) error {
	// Phase 1: Version Management
	o.logger.Info("Phase 1: Setting up version management")
	if err := o.executeVersionManagement(ctx); err != nil {
		return fmt.Errorf("version management phase failed: %w", err)
	}

	// Phase 2: Code Generation
	o.logger.Info("Phase 2: Generating code for all versions")
	if err := o.executeCodeGeneration(ctx); err != nil {
		return fmt.Errorf("code generation phase failed: %w", err)
	}

	// Phase 3: Analysis
	o.logger.Info("Phase 3: Analyzing cross-version differences")
	if err := o.executeAnalysis(ctx); err != nil {
		return fmt.Errorf("analysis phase failed: %w", err)
	}

	// Phase 4: Reporting
	o.logger.Info("Phase 4: Generating comprehensive reports")
	if err := o.executeReporting(ctx); err != nil {
		return fmt.Errorf("reporting phase failed: %w", err)
	}

	return nil
}

func (o *ValidationOrchestrator) executeVersionManagement(ctx context.Context) error {
	result, err := o.versionManager.Execute(ctx, nil)
	if err != nil {
		o.executionContext.Errors = append(o.executionContext.Errors, err)
		return err
	}

	o.executionContext.Results["version_management"] = result
	o.logger.Info("Version management completed successfully")
	return nil
}

func (o *ValidationOrchestrator) executeCodeGeneration(ctx context.Context) error {
	result, err := o.codeGenerator.Execute(ctx, nil)
	if err != nil {
		o.executionContext.Errors = append(o.executionContext.Errors, err)
		return err
	}

	o.executionContext.Results["code_generation"] = result
	o.logger.Info("Code generation completed successfully")
	return nil
}

func (o *ValidationOrchestrator) executeAnalysis(ctx context.Context) error {
	result, err := o.analysisAgent.Execute(ctx, nil)
	if err != nil {
		o.executionContext.Errors = append(o.executionContext.Errors, err)
		return err
	}

	o.executionContext.Results["analysis"] = result
	o.logger.Info("Analysis completed successfully")
	return nil
}

func (o *ValidationOrchestrator) executeReporting(ctx context.Context) error {
	result, err := o.reportingAgent.Execute(ctx, nil)
	if err != nil {
		o.executionContext.Errors = append(o.executionContext.Errors, err)
		return err
	}

	o.executionContext.Results["reporting"] = result
	o.logger.Info("Reporting completed successfully")
	return nil
}

// Validation Gates - as specified in PRP

func (o *ValidationOrchestrator) ValidateSetup(ctx context.Context) error {
	o.logger.Info("Running setup validation gate")

	// Check workspace directory
	if err := o.versionManager.EnsureDir(o.config.WorkspaceDir); err != nil {
		return fmt.Errorf("workspace directory validation failed: %w", err)
	}

	// Validate swagger file
	if !filepath.IsAbs(o.config.SwaggerFile) {
		abs, err := filepath.Abs(o.config.SwaggerFile)
		if err != nil {
			return fmt.Errorf("failed to resolve swagger file path: %w", err)
		}
		o.config.SwaggerFile = abs
	}

	// Validate agents
	agents := []types.Agent{
		o.versionManager,
		o.codeGenerator,
		o.analysisAgent,
		o.reportingAgent,
	}

	for _, agent := range agents {
		if err := agent.Validate(); err != nil {
			return fmt.Errorf("agent %s validation failed: %w", agent.Name(), err)
		}
	}

	o.logger.Info("✓ Setup validation passed")
	return nil
}

func (o *ValidationOrchestrator) ValidateGeneration(ctx context.Context) error {
	o.logger.Info("Running code generation validation gate")

	results := o.codeGenerator.GetGenerationResults()
	if len(results) == 0 {
		return fmt.Errorf("no generation results available")
	}

	for version, result := range results {
		if !result.Success {
			return fmt.Errorf("generation failed for version %s: %s", version, result.Error)
		}

		if len(result.GeneratedFiles) == 0 {
			return fmt.Errorf("no files generated for version %s", version)
		}

		o.logger.Info("✓ Version %s generated %d files", version, len(result.GeneratedFiles))
	}

	o.logger.Info("✓ Code generation validation passed")
	return nil
}

func (o *ValidationOrchestrator) ValidateAnalysis(ctx context.Context) error {
	o.logger.Info("Running analysis validation gate")

	results := o.analysisAgent.GetAnalysisResults()
	if len(results) == 0 {
		return fmt.Errorf("no analysis results available")
	}

	expectedComparisons := (len(o.config.Versions) * (len(o.config.Versions) - 1)) / 2
	if len(results) != expectedComparisons {
		return fmt.Errorf("expected %d comparisons, got %d", expectedComparisons, len(results))
	}

	for _, result := range results {
		if result.Summary == "" {
			return fmt.Errorf("analysis result missing summary for %s vs %s", result.Version1, result.Version2)
		}
		o.logger.Info("✓ Analysis: %s vs %s (%.1f%% compatible)", 
			result.Version1, result.Version2, result.CompatibilityScore)
	}

	o.logger.Info("✓ Analysis validation passed")
	return nil
}

func (o *ValidationOrchestrator) ValidateReporting(ctx context.Context) error {
	o.logger.Info("Running report generation validation gate")

	result, exists := o.executionContext.Results["reporting"]
	if !exists {
		return fmt.Errorf("reporting results not available")
	}

	generatedFiles, ok := result.(map[string]string)
	if !ok {
		return fmt.Errorf("invalid reporting results format")
	}

	if len(generatedFiles) == 0 {
		return fmt.Errorf("no report files generated")
	}

	for format, filename := range generatedFiles {
		o.logger.Info("✓ Generated %s report: %s", format, filename)
	}

	o.logger.Info("✓ Report generation validation passed")
	return nil
}

func (o *ValidationOrchestrator) GetExecutionContext() *types.ExecutionContext {
	return o.executionContext
}

func (o *ValidationOrchestrator) GetResults() map[string]interface{} {
	return o.executionContext.Results
}

func (o *ValidationOrchestrator) GetErrors() []error {
	return o.executionContext.Errors
}

// Public methods to expose individual pipeline phases

func (o *ValidationOrchestrator) ExecuteVersionManagement(ctx context.Context) error {
	return o.executeVersionManagement(ctx)
}

func (o *ValidationOrchestrator) ExecuteCodeGeneration(ctx context.Context) error {
	return o.executeCodeGeneration(ctx)
}

func (o *ValidationOrchestrator) ExecuteAnalysis(ctx context.Context) error {
	return o.executeAnalysis(ctx)
}

func (o *ValidationOrchestrator) ExecuteReporting(ctx context.Context) error {
	return o.executeReporting(ctx)
}

// Cleanup resources
func (o *ValidationOrchestrator) Cleanup() error {
	o.logger.Info("Cleaning up orchestrator resources")
	
	if err := o.versionManager.Cleanup(); err != nil {
		o.logger.Warn("Version manager cleanup warning: %v", err)
	}

	return nil
}