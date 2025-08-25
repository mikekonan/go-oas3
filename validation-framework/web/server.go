package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mikekonan/go-oas3/validation-framework/orchestrator"
	"github.com/mikekonan/go-oas3/validation-framework/types"
)

type ValidationServer struct {
	port           int
	logger         types.Logger
	orchestrator   *orchestrator.ValidationOrchestrator
	workspaceDir   string
	server         *http.Server
	isRunning      bool
}

type DashboardData struct {
	Title            string                    `json:"title"`
	LastRun          *time.Time               `json:"last_run,omitempty"`
	TotalValidations int                       `json:"total_validations"`
	RecentReports    []ReportSummary          `json:"recent_reports"`
	SystemInfo       SystemInfo               `json:"system_info"`
	QuickStats       QuickStats               `json:"quick_stats"`
}

type ReportSummary struct {
	Date               time.Time `json:"date"`
	VersionsCompared   []string  `json:"versions_compared"`
	CompatibilityScore float64   `json:"compatibility_score"`
	BreakingChanges    int       `json:"breaking_changes"`
	Status             string    `json:"status"`
	ReportPath         string    `json:"report_path"`
}

type SystemInfo struct {
	Version       string `json:"version"`
	GoVersion     string `json:"go_version"`
	Platform      string `json:"platform"`
	WorkspaceDir  string `json:"workspace_dir"`
	ConfiguredVersions []string `json:"configured_versions"`
}

type QuickStats struct {
	TotalReports       int     `json:"total_reports"`
	AverageCompatibility float64 `json:"average_compatibility"`
	LastRunDuration    string  `json:"last_run_duration"`
	TotalBreakingChanges int   `json:"total_breaking_changes"`
}

type ValidationRequest struct {
	Versions    []string `json:"versions"`
	SwaggerFile string   `json:"swagger_file"`
	Formats     []string `json:"formats"`
	Parallel    bool     `json:"parallel"`
}

type ValidationResponse struct {
	Success     bool                      `json:"success"`
	Message     string                    `json:"message"`
	Results     []types.AnalysisResult   `json:"results,omitempty"`
	Reports     []string                 `json:"reports,omitempty"`
	Duration    string                   `json:"duration"`
	Error       string                   `json:"error,omitempty"`
}

func NewValidationServer(port int, logger types.Logger, orch *orchestrator.ValidationOrchestrator, workspaceDir string) *ValidationServer {
	return &ValidationServer{
		port:         port,
		logger:       logger,
		orchestrator: orch,
		workspaceDir: workspaceDir,
	}
}

func (vs *ValidationServer) Start() error {
	mux := http.NewServeMux()
	
	// Static files
	mux.HandleFunc("/static/", vs.handleStatic)
	
	// Dashboard
	mux.HandleFunc("/", vs.handleDashboard)
	mux.HandleFunc("/dashboard", vs.handleDashboard)
	
	// API endpoints
	mux.HandleFunc("/api/validate", vs.handleValidate)
	mux.HandleFunc("/api/status", vs.handleStatus)
	mux.HandleFunc("/api/reports", vs.handleReports)
	mux.HandleFunc("/api/report/", vs.handleReport)
	mux.HandleFunc("/api/config", vs.handleConfig)
	mux.HandleFunc("/api/versions", vs.handleVersions)
	
	// Real-time updates
	mux.HandleFunc("/api/ws", vs.handleWebSocket)
	
	vs.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", vs.port),
		Handler: mux,
	}
	
	vs.isRunning = true
	vs.logger.Info("🌐 Starting validation web interface on http://localhost:%d", vs.port)
	
	return vs.server.ListenAndServe()
}

func (vs *ValidationServer) Stop() error {
	vs.isRunning = false
	if vs.server != nil {
		return vs.server.Close()
	}
	return nil
}

func (vs *ValidationServer) IsRunning() bool {
	return vs.isRunning
}

func (vs *ValidationServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	dashboardData := vs.getDashboardData()
	
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>go-oas3 Validation Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            color: #333;
        }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        .header { 
            background: rgba(255,255,255,0.95); 
            padding: 30px; 
            border-radius: 15px; 
            margin-bottom: 30px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.1);
            backdrop-filter: blur(10px);
        }
        .header h1 { 
            font-size: 2.5em; 
            color: #2c3e50; 
            margin-bottom: 10px;
            background: linear-gradient(45deg, #667eea, #764ba2);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }
        .header p { color: #7f8c8d; font-size: 1.1em; }
        .grid { 
            display: grid; 
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); 
            gap: 20px;
            margin-bottom: 30px;
        }
        .card { 
            background: rgba(255,255,255,0.95); 
            padding: 25px; 
            border-radius: 15px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.1);
            backdrop-filter: blur(10px);
            transition: all 0.3s ease;
        }
        .card:hover { transform: translateY(-5px); box-shadow: 0 15px 40px rgba(0,0,0,0.15); }
        .card h3 { color: #2c3e50; margin-bottom: 15px; font-size: 1.3em; }
        .stat-value { font-size: 2.5em; font-weight: bold; color: #27ae60; margin-bottom: 5px; }
        .stat-label { color: #7f8c8d; font-size: 0.9em; }
        .btn {
            background: linear-gradient(45deg, #667eea, #764ba2);
            color: white;
            padding: 12px 25px;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-size: 1em;
            transition: all 0.3s ease;
            text-decoration: none;
            display: inline-block;
            margin: 5px;
        }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 5px 15px rgba(0,0,0,0.2); }
        .btn-secondary { background: linear-gradient(45deg, #95a5a6, #7f8c8d); }
        .form-group { margin-bottom: 20px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: 600; color: #2c3e50; }
        .form-control { 
            width: 100%; 
            padding: 12px; 
            border: 2px solid #ecf0f1; 
            border-radius: 8px; 
            font-size: 1em;
            transition: border-color 0.3s ease;
        }
        .form-control:focus { 
            outline: none; 
            border-color: #667eea; 
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }
        .status-indicator {
            display: inline-block;
            width: 12px;
            height: 12px;
            border-radius: 50%;
            margin-right: 8px;
        }
        .status-success { background-color: #27ae60; }
        .status-warning { background-color: #f39c12; }
        .status-error { background-color: #e74c3c; }
        .recent-reports { margin-top: 20px; }
        .report-item {
            background: #f8f9fa;
            padding: 15px;
            margin-bottom: 10px;
            border-radius: 8px;
            border-left: 4px solid #667eea;
        }
        .report-item h4 { color: #2c3e50; margin-bottom: 8px; }
        .report-item p { color: #7f8c8d; margin-bottom: 5px; }
        .progress-bar {
            background: #ecf0f1;
            border-radius: 10px;
            height: 20px;
            overflow: hidden;
            margin: 10px 0;
        }
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            border-radius: 10px;
            transition: width 0.5s ease;
        }
        .loading { display: none; }
        .loading.active { display: inline-block; }
        @keyframes spin { 0% { transform: rotate(0deg); } 100% { transform: rotate(360deg); } }
        .spinner {
            border: 2px solid #f3f3f3;
            border-top: 2px solid #667eea;
            border-radius: 50%;
            width: 20px;
            height: 20px;
            animation: spin 1s linear infinite;
            display: inline-block;
            margin-left: 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 go-oas3 Validation Framework</h1>
            <p>Enterprise-grade cross-version compatibility validation for OpenAPI code generators</p>
        </div>
        
        <div class="grid">
            <div class="card">
                <h3>📊 Quick Stats</h3>
                <div class="stat-value">{{.QuickStats.TotalReports}}</div>
                <div class="stat-label">Total Reports</div>
                <div style="margin-top: 15px;">
                    <div class="stat-value" style="font-size: 1.8em; color: #667eea;">{{printf "%.1f%%" .QuickStats.AverageCompatibility}}</div>
                    <div class="stat-label">Average Compatibility</div>
                </div>
            </div>
            
            <div class="card">
                <h3>⚡ System Status</h3>
                <p><span class="status-indicator status-success"></span> Framework Online</p>
                <p><span class="status-indicator status-success"></span> Workspace: {{.SystemInfo.WorkspaceDir}}</p>
                <p><span class="status-indicator status-success"></span> Versions: {{len .SystemInfo.ConfiguredVersions}} configured</p>
                {{if .LastRun}}
                <p style="margin-top: 15px; color: #7f8c8d;">Last run: {{.LastRun.Format "2006-01-02 15:04:05"}}</p>
                {{end}}
            </div>
            
            <div class="card">
                <h3>🔧 Quick Actions</h3>
                <button class="btn" onclick="showValidationForm()">Run New Validation</button>
                <a href="/api/reports" class="btn btn-secondary">View Reports</a>
                <button class="btn btn-secondary" onclick="refreshDashboard()">
                    Refresh <span class="loading spinner"></span>
                </button>
            </div>
        </div>
        
        <div class="card" id="validation-form" style="display: none; margin-bottom: 30px;">
            <h3>🚀 Run Cross-Version Validation</h3>
            <form onsubmit="runValidation(event)">
                <div class="form-group">
                    <label>Versions to Compare:</label>
                    <input type="text" class="form-control" id="versions" placeholder="v1.0.63,v1.0.65" value="v1.0.63,v1.0.65">
                    <small style="color: #7f8c8d;">Comma-separated list of versions</small>
                </div>
                <div class="form-group">
                    <label>Swagger File Path:</label>
                    <input type="text" class="form-control" id="swagger-file" placeholder="/path/to/swagger.yaml" value="/Users/enquix/work/go-oas3/example/swagger.yaml">
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" id="parallel" checked> Run in parallel
                    </label>
                </div>
                <button type="submit" class="btn">
                    Run Validation <span class="loading spinner" id="validation-spinner"></span>
                </button>
                <button type="button" class="btn btn-secondary" onclick="hideValidationForm()">Cancel</button>
            </form>
            <div id="validation-result" style="margin-top: 20px; display: none;"></div>
        </div>
        
        {{if .RecentReports}}
        <div class="card">
            <h3>📋 Recent Reports</h3>
            <div class="recent-reports">
                {{range .RecentReports}}
                <div class="report-item">
                    <h4>{{range $i, $v := .VersionsCompared}}{{if $i}} vs {{end}}{{$v}}{{end}}</h4>
                    <p>Compatibility: <strong>{{printf "%.1f%%" .CompatibilityScore}}</strong></p>
                    <p>Breaking Changes: <strong>{{.BreakingChanges}}</strong></p>
                    <p>Generated: {{.Date.Format "2006-01-02 15:04:05"}}</p>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: {{.CompatibilityScore}}%"></div>
                    </div>
                </div>
                {{end}}
            </div>
        </div>
        {{end}}
    </div>
    
    <script>
        function showValidationForm() {
            document.getElementById('validation-form').style.display = 'block';
            document.getElementById('validation-form').scrollIntoView({behavior: 'smooth'});
        }
        
        function hideValidationForm() {
            document.getElementById('validation-form').style.display = 'none';
        }
        
        function refreshDashboard() {
            const spinner = document.querySelector('.loading');
            spinner.classList.add('active');
            setTimeout(() => {
                location.reload();
            }, 500);
        }
        
        async function runValidation(event) {
            event.preventDefault();
            
            const spinner = document.getElementById('validation-spinner');
            const resultDiv = document.getElementById('validation-result');
            
            spinner.classList.add('active');
            resultDiv.style.display = 'none';
            
            const versions = document.getElementById('versions').value.split(',').map(v => v.trim());
            const swaggerFile = document.getElementById('swagger-file').value;
            const parallel = document.getElementById('parallel').checked;
            
            try {
                const response = await fetch('/api/validate', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        versions: versions,
                        swagger_file: swaggerFile,
                        formats: ['markdown', 'json'],
                        parallel: parallel
                    })
                });
                
                const result = await response.json();
                
                resultDiv.innerHTML = result.success 
                    ? '<div style="color: #27ae60; padding: 15px; background: #d4edda; border-radius: 8px;"><h4>✅ Validation Completed Successfully!</h4><p>Duration: ' + result.duration + '</p><p>Reports: ' + result.reports.join(', ') + '</p></div>'
                    : '<div style="color: #e74c3c; padding: 15px; background: #f8d7da; border-radius: 8px;"><h4>❌ Validation Failed</h4><p>' + result.error + '</p></div>';
                
                resultDiv.style.display = 'block';
                
                if (result.success) {
                    setTimeout(() => {
                        location.reload();
                    }, 2000);
                }
                
            } catch (error) {
                resultDiv.innerHTML = '<div style="color: #e74c3c; padding: 15px; background: #f8d7da; border-radius: 8px;"><h4>❌ Network Error</h4><p>' + error.message + '</p></div>';
                resultDiv.style.display = 'block';
            } finally {
                spinner.classList.remove('active');
            }
        }
        
        // Auto-refresh dashboard every 30 seconds
        setInterval(() => {
            if (!document.getElementById('validation-spinner').classList.contains('active')) {
                location.reload();
            }
        }, 30000);
    </script>
</body>
</html>`
	
	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, dashboardData)
}

func (vs *ValidationServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	startTime := time.Now()
	
	// Create temporary config for this validation
	config := &types.ValidationConfig{
		Versions:      req.Versions,
		SwaggerFile:   req.SwaggerFile,
		WorkspaceDir:  vs.workspaceDir,
		OutputFormats: req.Formats,
		Parallel:      req.Parallel,
		Timeout:       30 * time.Minute,
	}
	
	// Run validation using orchestrator
	vs.logger.Info("Web API: Starting validation for versions: %v", req.Versions)
	
	// Create a new orchestrator for this request
	ctx := context.TODO()
	orch, err := orchestrator.NewValidationOrchestrator(config)
	if err != nil {
		response := ValidationResponse{
			Success:  false,
			Error:    err.Error(),
			Message:  "Failed to create orchestrator",
			Duration: time.Since(startTime).String(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	defer orch.Cleanup()

	// Execute full validation pipeline
	err = orch.Execute(ctx)
	duration := time.Since(startTime)
	
	response := ValidationResponse{
		Duration: duration.String(),
	}
	
	if err != nil {
		response.Success = false
		response.Error = err.Error()
		response.Message = "Validation failed"
	} else {
		response.Success = true
		response.Message = "Validation completed successfully"
		// Get results from orchestrator
		if analysisResults, ok := orch.GetResults()["analysis"]; ok {
			if results, ok := analysisResults.([]types.AnalysisResult); ok {
				response.Results = results
			}
		}
		response.Reports = vs.getGeneratedReports()
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (vs *ValidationServer) getDashboardData() DashboardData {
	recentReports := vs.getRecentReports()
	
	var totalBreaking int
	var totalCompatibility float64
	
	for _, report := range recentReports {
		totalBreaking += report.BreakingChanges
		totalCompatibility += report.CompatibilityScore
	}
	
	var avgCompatibility float64
	if len(recentReports) > 0 {
		avgCompatibility = totalCompatibility / float64(len(recentReports))
	}
	
	var lastRun *time.Time
	if len(recentReports) > 0 {
		lastRun = &recentReports[0].Date
	}
	
	return DashboardData{
		Title:            "go-oas3 Validation Dashboard",
		LastRun:          lastRun,
		TotalValidations: len(recentReports),
		RecentReports:    recentReports,
		SystemInfo: SystemInfo{
			Version:      "v1.0.0",
			WorkspaceDir: vs.workspaceDir,
			ConfiguredVersions: []string{"v1.0.63", "v1.0.65"},
		},
		QuickStats: QuickStats{
			TotalReports:         len(recentReports),
			AverageCompatibility: avgCompatibility,
			LastRunDuration:      "2.5s",
			TotalBreakingChanges: totalBreaking,
		},
	}
}

func (vs *ValidationServer) getRecentReports() []ReportSummary {
	reportsDir := filepath.Join(vs.workspaceDir, "analysis", "reports")
	
	// Mock data for now - in real implementation would scan actual reports
	return []ReportSummary{
		{
			Date:               time.Now().Add(-1 * time.Hour),
			VersionsCompared:   []string{"v1.0.63", "v1.0.65"},
			CompatibilityScore: 100.0,
			BreakingChanges:    0,
			Status:             "success",
			ReportPath:         filepath.Join(reportsDir, "validation-report.json"),
		},
	}
}

func (vs *ValidationServer) getGeneratedReports() []string {
	reportsDir := filepath.Join(vs.workspaceDir, "analysis", "reports")
	var reports []string
	
	files := []string{"validation-report.json", "validation-report.md", "compatibility-matrix.html"}
	for _, file := range files {
		path := filepath.Join(reportsDir, file)
		if _, err := os.Stat(path); err == nil {
			reports = append(reports, path)
		}
	}
	
	return reports
}

func (vs *ValidationServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"running":    vs.isRunning,
		"timestamp":  time.Now(),
		"workspace":  vs.workspaceDir,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (vs *ValidationServer) handleReports(w http.ResponseWriter, r *http.Request) {
	reports := vs.getRecentReports()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports)
}

func (vs *ValidationServer) handleReport(w http.ResponseWriter, r *http.Request) {
	// Extract report ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/report/")
	
	reportPath := filepath.Join(vs.workspaceDir, "analysis", "reports", path)
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}
	
	http.ServeFile(w, r, reportPath)
}

func (vs *ValidationServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"supported_versions": []string{"v1.0.63", "v1.0.65", "v1.0.66"},
		"default_formats":    []string{"markdown", "json", "html"},
		"workspace_dir":      vs.workspaceDir,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (vs *ValidationServer) handleVersions(w http.ResponseWriter, r *http.Request) {
	versions := []map[string]interface{}{
		{"version": "v1.0.63", "status": "available", "installed": true},
		{"version": "v1.0.65", "status": "available", "installed": true},
		{"version": "v1.0.66", "status": "unavailable", "installed": false},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}

func (vs *ValidationServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket implementation for real-time updates would go here
	// For now, return a placeholder
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "WebSocket endpoint - real-time updates coming soon!",
	})
}

func (vs *ValidationServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Serve static files (CSS, JS, images)
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	staticFile := filepath.Join("web", "static", path)
	
	if _, err := os.Stat(staticFile); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}
	
	http.ServeFile(w, r, staticFile)
}