package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	textTemplate "text/template"
	"time"

	"github.com/mikekonan/go-oas3/validation-framework/types"
)

type ReportingAgent struct {
	*BaseAgent
	analysisAgent   *AnalysisAgent
	codeGenerator   *CodeGeneratorAgent
	versionManager  *VersionManagerAgent
	outputFormats   []types.ReportFormat
	reportsDir      string
}

func NewReportingAgent(config *types.ValidationConfig, analysisAgent *AnalysisAgent, 
	codeGenerator *CodeGeneratorAgent, versionManager *VersionManagerAgent) *ReportingAgent {
	base := NewBaseAgent("reporting", "Reporting Agent", config)
	
	formats := []types.ReportFormat{types.FormatMarkdown, types.FormatJSON}
	if len(config.OutputFormats) > 0 {
		formats = make([]types.ReportFormat, len(config.OutputFormats))
		for i, f := range config.OutputFormats {
			formats[i] = types.ReportFormat(f)
		}
	}

	return &ReportingAgent{
		BaseAgent:      base,
		analysisAgent:  analysisAgent,
		codeGenerator:  codeGenerator,
		versionManager: versionManager,
		outputFormats:  formats,
		reportsDir:     filepath.Join(config.WorkspaceDir, "analysis", "reports"),
	}
}

func (r *ReportingAgent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	r.logger.Info("Starting report generation")

	// Ensure reports directory exists
	if err := r.EnsureDir(r.reportsDir); err != nil {
		return nil, fmt.Errorf("failed to create reports directory: %w", err)
	}

	// Generate comprehensive report
	report, err := r.generateReport()
	if err != nil {
		return nil, fmt.Errorf("failed to generate report: %w", err)
	}

	// Generate reports in all requested formats
	generatedFiles := make(map[string]string)
	for _, format := range r.outputFormats {
		filename, err := r.generateReportInFormat(report, format)
		if err != nil {
			r.logger.Error("Failed to generate %s report: %v", format, err)
			continue
		}
		generatedFiles[string(format)] = filename
	}

	// Also generate compatibility matrix HTML
	matrixFile, err := r.generateCompatibilityMatrix(report)
	if err != nil {
		r.logger.Warn("Failed to generate compatibility matrix: %v", err)
	} else {
		generatedFiles["compatibility-matrix"] = matrixFile
	}

	r.logger.Info("Successfully generated %d report files", len(generatedFiles))
	return generatedFiles, nil
}

func (r *ReportingAgent) generateReport() (*types.Report, error) {
	analysisResults := r.analysisAgent.GetAnalysisResults()
	generationResults := r.codeGenerator.GetGenerationResults()
	versionInfos := r.versionManager.GetInstalledVersions()

	report := &types.Report{
		Title:       "go-oas3 Cross-Version Validation Report",
		GeneratedAt: time.Now(),
		Versions:    r.config.Versions,
		Analyses:    analysisResults,
		CompatibilityMatrix: r.buildCompatibilityMatrix(analysisResults),
		Metadata:    make(map[string]interface{}),
	}

	// Generate summary
	report.Summary = r.generateSummary(analysisResults, generationResults, versionInfos)

	// Generate recommendations
	report.Recommendations = r.generateRecommendations(analysisResults)

	// Add metadata
	report.Metadata["swagger_file"] = r.config.SwaggerFile
	report.Metadata["workspace_dir"] = r.config.WorkspaceDir
	report.Metadata["generation_results"] = generationResults
	report.Metadata["version_infos"] = versionInfos

	return report, nil
}

func (r *ReportingAgent) buildCompatibilityMatrix(analyses []types.AnalysisResult) map[string]map[string]float64 {
	matrix := make(map[string]map[string]float64)
	
	// Initialize matrix
	for _, version := range r.config.Versions {
		matrix[version] = make(map[string]float64)
		for _, otherVersion := range r.config.Versions {
			if version == otherVersion {
				matrix[version][otherVersion] = 100.0
			} else {
				matrix[version][otherVersion] = 0.0
			}
		}
	}

	// Fill in compatibility scores
	for _, analysis := range analyses {
		matrix[analysis.Version1][analysis.Version2] = analysis.CompatibilityScore
		matrix[analysis.Version2][analysis.Version1] = analysis.CompatibilityScore
	}

	return matrix
}

func (r *ReportingAgent) generateSummary(analyses []types.AnalysisResult, 
	generations map[string]*types.GenerationResult, 
	versions map[string]*types.VersionInfo) types.ReportSummary {
	
	totalComparisons := len(analyses)
	breakingChanges := 0
	compatibleChanges := 0
	totalCompatibility := 0.0

	for _, analysis := range analyses {
		breakingChanges += len(analysis.BreakingChanges)
		
		totalDiffs := len(analysis.StructDifferences) + 
			         len(analysis.FunctionDifferences) + 
			         len(analysis.ImportDifferences) + 
			         len(analysis.ValidationDifferences)
		
		compatibleChanges += totalDiffs - len(analysis.BreakingChanges)
		totalCompatibility += analysis.CompatibilityScore
	}

	avgCompatibility := 0.0
	if totalComparisons > 0 {
		avgCompatibility = totalCompatibility / float64(totalComparisons)
	}

	status := "EXCELLENT"
	if avgCompatibility < 50 {
		status = "POOR"
	} else if avgCompatibility < 75 {
		status = "MODERATE"
	} else if avgCompatibility < 90 {
		status = "GOOD"
	}

	return types.ReportSummary{
		TotalComparisons:     totalComparisons,
		BreakingChangesFound: breakingChanges,
		CompatibleChanges:    compatibleChanges,
		AverageCompatibility: avgCompatibility,
		OverallStatus:        status,
	}
}

func (r *ReportingAgent) generateRecommendations(analyses []types.AnalysisResult) []string {
	var recommendations []string

	// Analysis-based recommendations
	highBreakingChanges := false
	lowCompatibility := false

	for _, analysis := range analyses {
		if len(analysis.BreakingChanges) > 5 {
			highBreakingChanges = true
		}
		if analysis.CompatibilityScore < 70 {
			lowCompatibility = true
		}
	}

	if highBreakingChanges {
		recommendations = append(recommendations, 
			"⚠️  High number of breaking changes detected. Consider implementing a deprecation strategy.")
	}

	if lowCompatibility {
		recommendations = append(recommendations, 
			"📋 Low compatibility scores indicate significant API changes. Review migration guide requirements.")
	}

	// Version-specific recommendations
	if len(r.config.Versions) >= 3 {
		recommendations = append(recommendations, 
			"🔄 Consider implementing automated compatibility testing in CI/CD pipeline.")
	}

	// Default recommendations
	if len(recommendations) == 0 {
		recommendations = append(recommendations, 
			"✅ Compatibility analysis completed successfully with no major concerns.")
	}

	recommendations = append(recommendations, 
		"📖 Review detailed analysis results for specific change impacts.",
		"🧪 Test generated code with existing applications before deploying.",
		"📊 Run this analysis regularly to track compatibility trends.")

	return recommendations
}

func (r *ReportingAgent) generateReportInFormat(report *types.Report, format types.ReportFormat) (string, error) {
	switch format {
	case types.FormatMarkdown:
		return r.generateMarkdownReport(report)
	case types.FormatJSON:
		return r.generateJSONReport(report)
	case types.FormatHTML:
		return r.generateHTMLReport(report)
	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
}

func (r *ReportingAgent) generateMarkdownReport(report *types.Report) (string, error) {
	filename := filepath.Join(r.reportsDir, "validation-report.md")
	
	tmplContent := `# {{.Title}}

**Generated:** {{.GeneratedAt.Format "2006-01-02 15:04:05"}}
**Versions Analyzed:** {{range $i, $v := .Versions}}{{if $i}}, {{end}}{{$v}}{{end}}

## Summary

- **Total Comparisons:** {{.Summary.TotalComparisons}}
- **Breaking Changes Found:** {{.Summary.BreakingChangesFound}}
- **Compatible Changes:** {{.Summary.CompatibleChanges}}
- **Average Compatibility:** {{printf "%.1f" .Summary.AverageCompatibility}}%
- **Overall Status:** {{.Summary.OverallStatus}}

## Compatibility Matrix

| Version | {{range .Versions}}{{.}} | {{end}}
|---------|{{range .Versions}}------|{{end}}
{{range $v1 := .Versions}}| {{$v1}} | {{range $v2 := $.Versions}}{{if eq $v1 $v2}}100.0% | {{else}}{{$score := index $.CompatibilityMatrix $v1 $v2}}{{printf "%.1f" $score}}% | {{end}}{{end}}
{{end}}

## Detailed Analysis Results

{{range .Analyses}}
### {{.Version1}} vs {{.Version2}}

**Compatibility Score:** {{printf "%.1f" .CompatibilityScore}}%
**Summary:** {{.Summary}}

{{if .StructDifferences}}
#### Struct Differences ({{len .StructDifferences}})
{{range .StructDifferences}}
- **{{.Name}}** ({{.ChangeType}}): {{.Description}}
{{range .FieldDiffs}}  - Field {{.Name}} ({{.ChangeType}}): {{.Impact}}
{{end}}{{end}}
{{end}}

{{if .FunctionDifferences}}
#### Function Differences ({{len .FunctionDifferences}})
{{range .FunctionDifferences}}
- **{{.Name}}** ({{.ChangeType}}): {{.Description}}
{{end}}
{{end}}

{{if .ImportDifferences}}
#### Import Differences ({{len .ImportDifferences}})
{{range .ImportDifferences}}
- **{{.Package}}** ({{.ChangeType}}): {{.Description}}
{{end}}
{{end}}

{{if .ValidationDifferences}}
#### Validation Differences ({{len .ValidationDifferences}})
{{range .ValidationDifferences}}
- **{{.Field}}** ({{.ChangeType}}): {{.Description}}
{{end}}
{{end}}

{{if .BreakingChanges}}
#### Breaking Changes ({{len .BreakingChanges}})
{{range .BreakingChanges}}
- **{{.Type}}** ({{.Severity}}): {{.Description}}
  - Impact: {{.Impact}}
{{end}}
{{end}}

---

{{end}}

## Recommendations

{{range .Recommendations}}
{{.}}

{{end}}

## Metadata

- **Swagger File:** {{index .Metadata "swagger_file"}}
- **Workspace Directory:** {{index .Metadata "workspace_dir"}}

---
*Report generated by go-oas3 Multi-Agent Cross-Version Validation Framework*
`

	tmpl, err := textTemplate.New("markdown").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse markdown template: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create markdown file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, report); err != nil {
		return "", fmt.Errorf("failed to execute markdown template: %w", err)
	}

	r.logger.Info("Generated markdown report: %s", filename)
	return filename, nil
}

func (r *ReportingAgent) generateJSONReport(report *types.Report) (string, error) {
	filename := filepath.Join(r.reportsDir, "validation-report.json")
	
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(report); err != nil {
		return "", fmt.Errorf("failed to encode JSON report: %w", err)
	}

	r.logger.Info("Generated JSON report: %s", filename)
	return filename, nil
}

func (r *ReportingAgent) generateHTMLReport(report *types.Report) (string, error) {
	filename := filepath.Join(r.reportsDir, "validation-report.html")
	
	tmplContent := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; }
        .summary { background-color: #e9ecef; padding: 15px; margin: 20px 0; border-radius: 5px; }
        .analysis { border: 1px solid #dee2e6; margin: 20px 0; padding: 15px; border-radius: 5px; }
        .breaking-changes { background-color: #f8d7da; padding: 10px; margin: 10px 0; border-radius: 3px; }
        .compatibility-score { font-size: 1.2em; font-weight: bold; }
        table { border-collapse: collapse; width: 100%; margin: 10px 0; }
        th, td { border: 1px solid #dee2e6; padding: 8px; text-align: center; }
        th { background-color: #e9ecef; }
        .score-excellent { color: #28a745; }
        .score-good { color: #ffc107; }
        .score-poor { color: #dc3545; }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.Title}}</h1>
        <p><strong>Generated:</strong> {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
        <p><strong>Versions:</strong> {{range $i, $v := .Versions}}{{if $i}}, {{end}}{{$v}}{{end}}</p>
    </div>

    <div class="summary">
        <h2>Summary</h2>
        <ul>
            <li><strong>Total Comparisons:</strong> {{.Summary.TotalComparisons}}</li>
            <li><strong>Breaking Changes:</strong> {{.Summary.BreakingChangesFound}}</li>
            <li><strong>Compatible Changes:</strong> {{.Summary.CompatibleChanges}}</li>
            <li><strong>Average Compatibility:</strong> <span class="compatibility-score">{{printf "%.1f" .Summary.AverageCompatibility}}%</span></li>
            <li><strong>Overall Status:</strong> {{.Summary.OverallStatus}}</li>
        </ul>
    </div>

    <h2>Compatibility Matrix</h2>
    <table>
        <tr>
            <th>Version</th>
            {{range .Versions}}<th>{{.}}</th>{{end}}
        </tr>
        {{range $v1 := .Versions}}
        <tr>
            <td><strong>{{$v1}}</strong></td>
            {{range $v2 := $.Versions}}
                {{if eq $v1 $v2}}
                    <td class="score-excellent">100.0%</td>
                {{else}}
                    {{$score := index $.CompatibilityMatrix $v1 $v2}}
                    <td class="{{if gt $score 90.0}}score-excellent{{else if gt $score 70.0}}score-good{{else}}score-poor{{end}}">{{printf "%.1f" $score}}%</td>
                {{end}}
            {{end}}
        </tr>
        {{end}}
    </table>

    <h2>Detailed Analysis</h2>
    {{range .Analyses}}
    <div class="analysis">
        <h3>{{.Version1}} vs {{.Version2}}</h3>
        <p><strong>Compatibility Score:</strong> <span class="compatibility-score">{{printf "%.1f" .CompatibilityScore}}%</span></p>
        <p>{{.Summary}}</p>
        
        {{if .BreakingChanges}}
        <div class="breaking-changes">
            <h4>Breaking Changes ({{len .BreakingChanges}})</h4>
            <ul>
                {{range .BreakingChanges}}
                <li><strong>{{.Type}}</strong> ({{.Severity}}): {{.Description}}</li>
                {{end}}
            </ul>
        </div>
        {{end}}
        
        {{if .StructDifferences}}
        <h4>Struct Differences ({{len .StructDifferences}})</h4>
        <ul>
            {{range .StructDifferences}}
            <li><strong>{{.Name}}</strong> ({{.ChangeType}}): {{.Description}}</li>
            {{end}}
        </ul>
        {{end}}
    </div>
    {{end}}

    <h2>Recommendations</h2>
    <ul>
        {{range .Recommendations}}
        <li>{{.}}</li>
        {{end}}
    </ul>
</body>
</html>`

	tmpl, err := template.New("html").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create HTML file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, report); err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %w", err)
	}

	r.logger.Info("Generated HTML report: %s", filename)
	return filename, nil
}

func (r *ReportingAgent) generateCompatibilityMatrix(report *types.Report) (string, error) {
	filename := filepath.Join(r.reportsDir, "compatibility-matrix.html")
	
	// Create a specialized compatibility matrix visualization
	tmplContent := `<!DOCTYPE html>
<html>
<head>
    <title>Compatibility Matrix - {{.Title}}</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; text-align: center; }
        .matrix { margin: 20px auto; }
        table { border-collapse: collapse; margin: 0 auto; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        th, td { padding: 15px; border: 1px solid #ddd; min-width: 80px; }
        th { background-color: #34495e; color: white; font-weight: bold; }
        .version-header { background-color: #2c3e50 !important; }
        .excellent { background-color: #27ae60; color: white; }
        .good { background-color: #f39c12; color: white; }
        .moderate { background-color: #e67e22; color: white; }
        .poor { background-color: #e74c3c; color: white; }
        .legend { margin: 20px 0; }
        .legend-item { display: inline-block; margin: 0 10px; padding: 5px 10px; color: white; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>Cross-Version Compatibility Matrix</h1>
    <p>Generated: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
    
    <div class="legend">
        <span class="legend-item excellent">Excellent (≥90%)</span>
        <span class="legend-item good">Good (70-89%)</span>
        <span class="legend-item moderate">Moderate (50-69%)</span>
        <span class="legend-item poor">Poor (<50%)</span>
    </div>
    
    <div class="matrix">
        <table>
            <tr>
                <th class="version-header">Version</th>
                {{range .Versions}}<th class="version-header">{{.}}</th>{{end}}
            </tr>
            {{range $v1 := .Versions}}
            <tr>
                <th class="version-header">{{$v1}}</th>
                {{range $v2 := $.Versions}}
                    {{if eq $v1 $v2}}
                        <td class="excellent">100.0%</td>
                    {{else}}
                        {{$score := index $.CompatibilityMatrix $v1 $v2}}
                        <td class="{{if ge $score 90.0}}excellent{{else if ge $score 70.0}}good{{else if ge $score 50.0}}moderate{{else}}poor{{end}}">{{printf "%.1f" $score}}%</td>
                    {{end}}
                {{end}}
            </tr>
            {{end}}
        </table>
    </div>
    
    <div style="margin-top: 30px;">
        <h3>Overall Compatibility: {{printf "%.1f" .Summary.AverageCompatibility}}% - {{.Summary.OverallStatus}}</h3>
        <p>{{.Summary.BreakingChangesFound}} breaking changes found across {{.Summary.TotalComparisons}} comparisons</p>
    </div>
</body>
</html>`

	tmpl, err := template.New("matrix").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse matrix template: %w", err)
	}

	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create matrix file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, report); err != nil {
		return "", fmt.Errorf("failed to execute matrix template: %w", err)
	}

	r.logger.Info("Generated compatibility matrix: %s", filename)
	return filename, nil
}