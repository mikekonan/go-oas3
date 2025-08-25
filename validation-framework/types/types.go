package types

import (
	"context"
	"go/ast"
	"time"
)

type Agent interface {
	ID() string
	Name() string
	Execute(ctx context.Context, input interface{}) (interface{}, error)
	Validate() error
}

type ValidationConfig struct {
	Versions      []string                `json:"versions"`
	SwaggerFile   string                 `json:"swagger_file"`
	WorkspaceDir  string                 `json:"workspace_dir"`
	OutputFormats []string               `json:"output_formats"`
	Parallel      bool                   `json:"parallel"`
	Timeout       time.Duration          `json:"timeout"`
	BinaryPaths   map[string]string      `json:"binary_paths"`
}

type VersionInfo struct {
	Version    string    `json:"version"`
	BinaryPath string    `json:"binary_path"`
	Installed  bool      `json:"installed"`
	Validated  bool      `json:"validated"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type GenerationResult struct {
	Version      string            `json:"version"`
	OutputDir    string           `json:"output_dir"`
	GeneratedFiles []string       `json:"generated_files"`
	Success      bool             `json:"success"`
	Error        string           `json:"error,omitempty"`
	Duration     time.Duration    `json:"duration"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type AnalysisResult struct {
	Version1         string           `json:"version1"`
	Version2         string           `json:"version2"`
	StructDifferences []StructDiff    `json:"struct_differences"`
	FunctionDifferences []FunctionDiff `json:"function_differences"`
	ImportDifferences []ImportDiff    `json:"import_differences"`
	ValidationDifferences []ValidationDiff `json:"validation_differences"`
	CompatibilityScore float64        `json:"compatibility_score"`
	BreakingChanges   []BreakingChange `json:"breaking_changes"`
	Summary          string           `json:"summary"`
}

type StructDiff struct {
	Name        string      `json:"name"`
	ChangeType  ChangeType  `json:"change_type"`
	OldStruct   *StructInfo `json:"old_struct,omitempty"`
	NewStruct   *StructInfo `json:"new_struct,omitempty"`
	FieldDiffs  []FieldDiff `json:"field_diffs"`
	Description string      `json:"description"`
}

type FieldDiff struct {
	Name       string     `json:"name"`
	ChangeType ChangeType `json:"change_type"`
	OldType    string     `json:"old_type,omitempty"`
	NewType    string     `json:"new_type,omitempty"`
	OldTags    string     `json:"old_tags,omitempty"`
	NewTags    string     `json:"new_tags,omitempty"`
	Impact     string     `json:"impact"`
}

type FunctionDiff struct {
	Name        string     `json:"name"`
	ChangeType  ChangeType `json:"change_type"`
	OldSignature string    `json:"old_signature,omitempty"`
	NewSignature string    `json:"new_signature,omitempty"`
	Description string     `json:"description"`
}

type ImportDiff struct {
	Package    string     `json:"package"`
	ChangeType ChangeType `json:"change_type"`
	Alias      string     `json:"alias,omitempty"`
	Description string    `json:"description"`
}

type ValidationDiff struct {
	Field       string     `json:"field"`
	ChangeType  ChangeType `json:"change_type"`
	OldRule     string     `json:"old_rule,omitempty"`
	NewRule     string     `json:"new_rule,omitempty"`
	Impact      string     `json:"impact"`
	Description string     `json:"description"`
}

type BreakingChange struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Severity    string `json:"severity"`
}

type StructInfo struct {
	Name    string            `json:"name"`
	Fields  map[string]string `json:"fields"`
	Tags    map[string]string `json:"tags"`
	Methods []string          `json:"methods"`
}

type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeRemoved  ChangeType = "removed"
	ChangeModified ChangeType = "modified"
	ChangeRenamed  ChangeType = "renamed"
)

type ReportFormat string

const (
	FormatMarkdown ReportFormat = "markdown"
	FormatJSON     ReportFormat = "json"
	FormatHTML     ReportFormat = "html"
)

type Report struct {
	Title           string                    `json:"title"`
	GeneratedAt     time.Time                 `json:"generated_at"`
	Versions        []string                  `json:"versions"`
	Summary         ReportSummary             `json:"summary"`
	Analyses        []AnalysisResult          `json:"analyses"`
	CompatibilityMatrix map[string]map[string]float64 `json:"compatibility_matrix"`
	Recommendations []string                  `json:"recommendations"`
	Metadata        map[string]interface{}    `json:"metadata"`
}

type ReportSummary struct {
	TotalComparisons    int     `json:"total_comparisons"`
	BreakingChangesFound int    `json:"breaking_changes_found"`
	CompatibleChanges   int     `json:"compatible_changes"`
	AverageCompatibility float64 `json:"average_compatibility"`
	OverallStatus       string  `json:"overall_status"`
}

type ExecutionContext struct {
	Config    *ValidationConfig `json:"config"`
	StartTime time.Time         `json:"start_time"`
	Results   map[string]interface{} `json:"results"`
	Errors    []error           `json:"errors"`
	Logger    Logger            `json:"-"`
}

type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

type ASTInfo struct {
	File     *ast.File `json:"-"`
	Structs  map[string]*StructInfo `json:"structs"`
	Functions map[string]string     `json:"functions"`
	Imports  []string              `json:"imports"`
	Package  string                `json:"package"`
}

// Performance monitoring types
type PerformanceProfiler struct {
	Phase     string        `json:"phase"`
	StartTime time.Time     `json:"start_time"`
	StartMem  *MemoryStats  `json:"start_memory"`
}

type PerformanceMetrics struct {
	Phase            string         `json:"phase"`
	Duration         time.Duration  `json:"duration"`
	StartTime        time.Time      `json:"start_time"`
	EndTime          time.Time      `json:"end_time"`
	MemoryStart      *MemoryStats   `json:"memory_start"`
	MemoryEnd        *MemoryStats   `json:"memory_end"`
	MemoryDelta      uint64         `json:"memory_delta"`
	CPUUsage         float64        `json:"cpu_usage"`
	GoroutineCount   int            `json:"goroutine_count"`
	FileCount        int            `json:"file_count"`
	LinesGenerated   int            `json:"lines_generated"`
}

type MemoryStats struct {
	Alloc      uint64 `json:"alloc"`
	TotalAlloc uint64 `json:"total_alloc"`
	Sys        uint64 `json:"sys"`
	NumGC      uint32 `json:"num_gc"`
	HeapAlloc  uint64 `json:"heap_alloc"`
	HeapSys    uint64 `json:"heap_sys"`
}

type ResourceUsage struct {
	CPUCores       int    `json:"cpu_cores"`
	MemoryUsed     uint64 `json:"memory_used"`
	MemoryTotal    uint64 `json:"memory_total"`
	GoroutineCount int    `json:"goroutine_count"`
	DiskSpaceUsed  int64  `json:"disk_space_used"`
}

type ComparisonPerformance struct {
	Version1          string        `json:"version1"`
	Version2          string        `json:"version2"`
	Duration1         time.Duration `json:"duration1"`
	Duration2         time.Duration `json:"duration2"`
	PerformanceChange float64       `json:"performance_change"` // Percentage change
	Impact            string        `json:"impact"`
}