package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mikekonan/go-oas3/validation-framework/config"
	"github.com/mikekonan/go-oas3/validation-framework/orchestrator"
	"github.com/mikekonan/go-oas3/validation-framework/types"
	"github.com/mikekonan/go-oas3/validation-framework/web"
)

const (
	ExitSuccess = 0
	ExitError   = 1
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitError)
	}
}

func run(args []string) error {
	// Check for help or version flags
	if len(args) > 0 {
		switch args[0] {
		case "--help", "-h", "help":
			printUsage()
			return nil
		case "--version", "-v", "version":
			printVersion()
			return nil
		}
	}

	// Load configuration
	configLoader := config.NewConfigLoader()
	
	cfg, err := configLoader.LoadFromArgs(args)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := configLoader.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Print configuration
	fmt.Println("go-oas3 Multi-Agent Cross-Version Validation Framework")
	fmt.Println("====================================================")
	configLoader.PrintConfig(cfg)
	fmt.Println()

	// Check for specific commands
	if len(args) > 0 {
		switch args[0] {
		case "--validate-setup":
			return runValidateSetup(cfg)
		case "--generate-all":
			return runGenerateAll(cfg)
		case "--analyze":
			return runAnalyze(cfg)
		case "--report":
			return runReport(cfg)
		case "--web":
			return runWebInterface(cfg)
		case "--daemon":
			return runDaemon(cfg)
		}
	}

	// Run full validation pipeline
	return runFullValidation(cfg)
}

func runFullValidation(cfg *types.ValidationConfig) error {
	fmt.Println("🚀 Starting full validation pipeline...")
	
	// Create orchestrator
	orch, err := orchestrator.NewValidationOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Cleanup()

	// Setup validation gate
	fmt.Println("\n📋 Gate 1: Environment Setup Validation")
	ctx := context.Background()
	if err := orch.ValidateSetup(ctx); err != nil {
		return fmt.Errorf("setup validation failed: %w", err)
	}

	// Execute full pipeline
	fmt.Println("\n🏃 Executing validation pipeline...")
	if err := orch.Execute(ctx); err != nil {
		return fmt.Errorf("validation execution failed: %w", err)
	}

	// Run all validation gates
	fmt.Println("\n📋 Gate 2: Code Generation Validation")
	if err := orch.ValidateGeneration(ctx); err != nil {
		return fmt.Errorf("generation validation failed: %w", err)
	}

	fmt.Println("\n📋 Gate 3: Analysis Validation")
	if err := orch.ValidateAnalysis(ctx); err != nil {
		return fmt.Errorf("analysis validation failed: %w", err)
	}

	fmt.Println("\n📋 Gate 4: Report Generation Validation")
	if err := orch.ValidateReporting(ctx); err != nil {
		return fmt.Errorf("reporting validation failed: %w", err)
	}

	// Print summary
	printExecutionSummary(orch)
	
	fmt.Println("\n✅ Validation pipeline completed successfully!")
	return nil
}

func runValidateSetup(cfg *types.ValidationConfig) error {
	fmt.Println("🔍 Running setup validation...")
	
	orch, err := orchestrator.NewValidationOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Cleanup()

	if err := orch.ValidateSetup(context.Background()); err != nil {
		return err
	}

	fmt.Println("✅ Setup validation passed")
	return nil
}

func runGenerateAll(cfg *types.ValidationConfig) error {
	fmt.Println("🔨 Generating code for all versions...")
	
	orch, err := orchestrator.NewValidationOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Cleanup()

	ctx := context.Background()
	
	// Setup first
	if err := orch.ValidateSetup(ctx); err != nil {
		return fmt.Errorf("setup validation failed: %w", err)
	}

	// Execute version management and code generation phases only
	if err := orch.ExecuteVersionManagement(ctx); err != nil {
		return fmt.Errorf("version management failed: %w", err)
	}

	if err := orch.ExecuteCodeGeneration(ctx); err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	if err := orch.ValidateGeneration(ctx); err != nil {
		return fmt.Errorf("generation validation failed: %w", err)
	}

	fmt.Println("✅ Code generation completed successfully")
	return nil
}

func runAnalyze(cfg *types.ValidationConfig) error {
	fmt.Println("🔬 Running cross-version analysis...")
	
	orch, err := orchestrator.NewValidationOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Cleanup()

	ctx := context.Background()
	
	// Run through analysis phase
	if err := orch.ValidateSetup(ctx); err != nil {
		return fmt.Errorf("setup validation failed: %w", err)
	}

	if err := orch.ExecuteVersionManagement(ctx); err != nil {
		return fmt.Errorf("version management failed: %w", err)
	}

	if err := orch.ExecuteCodeGeneration(ctx); err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	if err := orch.ExecuteAnalysis(ctx); err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	if err := orch.ValidateAnalysis(ctx); err != nil {
		return fmt.Errorf("analysis validation failed: %w", err)
	}

	fmt.Println("✅ Analysis completed successfully")
	return nil
}

func runReport(cfg *types.ValidationConfig) error {
	fmt.Println("📊 Generating comprehensive reports...")
	
	orch, err := orchestrator.NewValidationOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Cleanup()

	ctx := context.Background()
	
	// Run full pipeline to ensure we have all data
	if err := orch.Execute(ctx); err != nil {
		return fmt.Errorf("validation execution failed: %w", err)
	}

	if err := orch.ValidateReporting(ctx); err != nil {
		return fmt.Errorf("reporting validation failed: %w", err)
	}

	// Print report locations
	results := orch.GetResults()
	if reportResults, ok := results["reporting"].(map[string]string); ok {
		fmt.Println("\n📋 Generated Reports:")
		for format, filename := range reportResults {
			fmt.Printf("  %s: %s\n", format, filename)
		}
	}

	fmt.Println("✅ Report generation completed successfully")
	return nil
}

func printExecutionSummary(orch *orchestrator.ValidationOrchestrator) {
	ctx := orch.GetExecutionContext()
	results := orch.GetResults()
	errors := orch.GetErrors()

	fmt.Println("\n📊 Execution Summary")
	fmt.Println("===================")
	fmt.Printf("Duration: %v\n", time.Since(ctx.StartTime))
	fmt.Printf("Versions: %v\n", ctx.Config.Versions)
	fmt.Printf("Phases Completed: %d\n", len(results))
	
	if len(errors) > 0 {
		fmt.Printf("Errors Encountered: %d\n", len(errors))
		for i, err := range errors {
			fmt.Printf("  %d. %v\n", i+1, err)
		}
	} else {
		fmt.Println("Errors: None")
	}

	// Print report locations if available
	if reportResults, ok := results["reporting"].(map[string]string); ok {
		fmt.Println("\n📋 Generated Reports:")
		for format, filename := range reportResults {
			fmt.Printf("  %s: %s\n", format, filename)
		}
	}
}

func printUsage() {
	fmt.Println(`go-oas3 Multi-Agent Cross-Version Validation Framework

USAGE:
    validation-framework [OPTIONS] [COMMAND]

COMMANDS:
    help                    Show this help message
    version                 Show version information
    --validate-setup        Validate environment setup only
    --generate-all          Generate code for all versions only  
    --analyze               Run analysis phase only
    --report                Generate reports only
    --web                   Start interactive web interface
    --daemon                Run as daemon with web interface
    (no command)            Run full validation pipeline

OPTIONS:
    --versions <versions>   Comma-separated list of versions (default: v1.0.63,v1.0.65,v1.0.66)
    --swagger-file <path>   Path to swagger/OpenAPI file (default: ./example/swagger.yaml)
    --workspace <dir>       Workspace directory (default: ./validation-workspace)
    --formats <formats>     Output formats: markdown,json,html (default: markdown,json)
    --timeout <duration>    Timeout duration (default: 30m)
    --sequential            Run code generation sequentially instead of parallel

EXAMPLES:
    # Run full validation with default settings
    ./validation-framework

    # Validate specific versions
    ./validation-framework --versions v1.0.63,v1.0.66

    # Use custom swagger file and workspace
    ./validation-framework --swagger-file ./my-api.yaml --workspace ./my-workspace

    # Generate all reports formats
    ./validation-framework --formats markdown,json,html

    # Just validate the setup
    ./validation-framework --validate-setup

    # Run analysis only
    ./validation-framework --analyze

VALIDATION GATES:
    Gate 1: Environment Setup - Validates workspace and tool installations
    Gate 2: Code Generation - Verifies all versions generate code successfully  
    Gate 3: Analysis - Ensures meaningful comparison results
    Gate 4: Reporting - Confirms all report formats generated properly

OUTPUT:
    - Generated code in <workspace>/generated/v<version>/
    - Analysis results in <workspace>/analysis/
    - Reports in <workspace>/analysis/reports/`)
}

func printVersion() {
	fmt.Println("go-oas3 Multi-Agent Cross-Version Validation Framework")
	fmt.Println("Version: 1.0.0")
	fmt.Println("Built for go-oas3 versions: v1.0.63, v1.0.65, v1.0.66")
}

func runWebInterface(cfg *types.ValidationConfig) error {
	fmt.Println("🌐 Starting go-oas3 Validation Web Interface...")
	
	// Create orchestrator for web API
	orch, err := orchestrator.NewValidationOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Cleanup()
	
	// Create and start web server
	logger := &simpleLogger{}
	webServer := web.NewValidationServer(8080, logger, orch, cfg.WorkspaceDir)
	
	fmt.Println("🚀 Web interface available at: http://localhost:8080")
	fmt.Println("📊 Dashboard, API endpoints, and real-time monitoring enabled")
	fmt.Println("Press Ctrl+C to stop the server")
	
	return webServer.Start()
}

func runDaemon(cfg *types.ValidationConfig) error {
	fmt.Println("🤖 Starting go-oas3 Validation Daemon...")
	fmt.Println("⚡ Daemon mode enables:")
	fmt.Println("   - Web interface on port 8080")
	fmt.Println("   - Scheduled validations")
	fmt.Println("   - File system monitoring")
	fmt.Println("   - API endpoints")
	
	// Create orchestrator
	orch, err := orchestrator.NewValidationOrchestrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Cleanup()
	
	// Start web server in background
	logger := &simpleLogger{}
	webServer := web.NewValidationServer(8080, logger, orch, cfg.WorkspaceDir)
	
	go func() {
		if err := webServer.Start(); err != nil {
			fmt.Printf("Web server error: %v\n", err)
		}
	}()
	
	fmt.Println("🌐 Web interface: http://localhost:8080")
	fmt.Println("🔄 Daemon running... Press Ctrl+C to stop")
	
	// Keep daemon running
	select {}
}

// Simple logger implementation
type simpleLogger struct{}

func (s *simpleLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO] "+msg+"\n", args...)
}

func (s *simpleLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}

func (s *simpleLogger) Debug(msg string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+msg+"\n", args...)
}

func (s *simpleLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN] "+msg+"\n", args...)
}