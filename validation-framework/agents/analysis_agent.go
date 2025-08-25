package agents

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mikekonan/go-oas3/validation-framework/types"
)

type AnalysisAgent struct {
	*BaseAgent
	codeGenerator *CodeGeneratorAgent
	parsedFiles   map[string]map[string]*types.ASTInfo // version -> file -> ASTInfo
	analysisResults []types.AnalysisResult
	mu            sync.RWMutex
}

func NewAnalysisAgent(config *types.ValidationConfig, codeGenerator *CodeGeneratorAgent) *AnalysisAgent {
	base := NewBaseAgent("analysis", "Analysis Agent", config)
	return &AnalysisAgent{
		BaseAgent:     base,
		codeGenerator: codeGenerator,
		parsedFiles:   make(map[string]map[string]*types.ASTInfo),
		analysisResults: make([]types.AnalysisResult, 0),
	}
}

func (a *AnalysisAgent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	a.logger.Info("Starting code analysis for all versions")

	// Parse all generated files
	if err := a.parseAllVersions(ctx); err != nil {
		return nil, fmt.Errorf("failed to parse generated files: %w", err)
	}

	// Perform cross-version analysis
	analysisResults, err := a.performCrossVersionAnalysis(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to perform analysis: %w", err)
	}

	a.mu.Lock()
	a.analysisResults = analysisResults
	a.mu.Unlock()

	a.logger.Info("Completed analysis of %d version comparisons", len(analysisResults))
	return analysisResults, nil
}

func (a *AnalysisAgent) parseAllVersions(ctx context.Context) error {
	for _, version := range a.config.Versions {
		if err := a.parseVersion(ctx, version); err != nil {
			return fmt.Errorf("failed to parse version %s: %w", version, err)
		}
	}
	return nil
}

func (a *AnalysisAgent) parseVersion(ctx context.Context, version string) error {
	a.logger.Info("Parsing generated files for version %s", version)

	outputDir, err := a.codeGenerator.GetOutputDir(version)
	if err != nil {
		return err
	}

	versionFiles := make(map[string]*types.ASTInfo)

	// Parse each generated file
	generatedFiles := []string{"components_gen.go", "routes_gen.go", "spec_gen.go"}
	for _, filename := range generatedFiles {
		filePath := filepath.Join(outputDir, filename)
		
		astInfo, err := a.parseGoFile(filePath)
		if err != nil {
			a.logger.Warn("Failed to parse %s for version %s: %v", filename, version, err)
			continue
		}

		versionFiles[filename] = astInfo
	}

	a.mu.Lock()
	a.parsedFiles[version] = versionFiles
	a.mu.Unlock()

	a.logger.Info("Successfully parsed %d files for version %s", len(versionFiles), version)
	return nil
}

func (a *AnalysisAgent) parseGoFile(filePath string) (*types.ASTInfo, error) {
	fileSet := token.NewFileSet()
	
	node, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	astInfo := &types.ASTInfo{
		File:      node,
		Structs:   make(map[string]*types.StructInfo),
		Functions: make(map[string]string),
		Imports:   make([]string, 0),
		Package:   node.Name.Name,
	}

	// Extract imports
	for _, importSpec := range node.Imports {
		importPath := strings.Trim(importSpec.Path.Value, `"`)
		if importSpec.Name != nil {
			importPath = importSpec.Name.Name + " " + importPath
		}
		astInfo.Imports = append(astInfo.Imports, importPath)
	}

	// Extract structs and functions
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if structType, ok := x.Type.(*ast.StructType); ok {
				structInfo := a.extractStructInfo(x.Name.Name, structType)
				astInfo.Structs[x.Name.Name] = structInfo
			}
		case *ast.FuncDecl:
			if x.Name != nil {
				funcName := x.Name.Name
				if x.Recv != nil && len(x.Recv.List) > 0 {
					// Include receiver type in function key for methods
					receiverType := a.typeToString(x.Recv.List[0].Type)
					funcName = receiverType + "." + funcName
				}
				signature := a.extractFunctionSignature(x)
				astInfo.Functions[funcName] = signature
			}
		}
		return true
	})

	return astInfo, nil
}

func (a *AnalysisAgent) extractStructInfo(name string, structType *ast.StructType) *types.StructInfo {
	structInfo := &types.StructInfo{
		Name:    name,
		Fields:  make(map[string]string),
		Tags:    make(map[string]string),
		Methods: make([]string, 0),
	}

	if structType.Fields != nil {
		for _, field := range structType.Fields.List {
			if len(field.Names) > 0 {
				fieldName := field.Names[0].Name
				fieldType := a.typeToString(field.Type)
				structInfo.Fields[fieldName] = fieldType
				
				if field.Tag != nil {
					tag := strings.Trim(field.Tag.Value, "`")
					structInfo.Tags[fieldName] = tag
				}
			}
		}
	}

	return structInfo
}

func (a *AnalysisAgent) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return a.typeToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + a.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + a.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + a.typeToString(t.Key) + "]" + a.typeToString(t.Value)
	default:
		return fmt.Sprintf("%T", t)
	}
}

func (a *AnalysisAgent) extractFunctionSignature(funcDecl *ast.FuncDecl) string {
	var parts []string
	
	if funcDecl.Recv != nil {
		parts = append(parts, "(receiver)")
	}
	
	parts = append(parts, funcDecl.Name.Name)
	
	if funcDecl.Type.Params != nil {
		var params []string
		for _, param := range funcDecl.Type.Params.List {
			paramType := a.typeToString(param.Type)
			if len(param.Names) > 0 {
				for _, name := range param.Names {
					params = append(params, name.Name+" "+paramType)
				}
			} else {
				params = append(params, paramType)
			}
		}
		parts = append(parts, "("+strings.Join(params, ", ")+")")
	}
	
	if funcDecl.Type.Results != nil {
		var results []string
		for _, result := range funcDecl.Type.Results.List {
			resultType := a.typeToString(result.Type)
			results = append(results, resultType)
		}
		if len(results) == 1 {
			parts = append(parts, results[0])
		} else {
			parts = append(parts, "("+strings.Join(results, ", ")+")")
		}
	}
	
	return strings.Join(parts, " ")
}

func (a *AnalysisAgent) performCrossVersionAnalysis(ctx context.Context) ([]types.AnalysisResult, error) {
	versions := a.config.Versions
	var results []types.AnalysisResult

	// Compare each pair of versions
	for i := 0; i < len(versions); i++ {
		for j := i + 1; j < len(versions); j++ {
			v1, v2 := versions[i], versions[j]
			
			result, err := a.compareVersions(v1, v2)
			if err != nil {
				a.logger.Error("Failed to compare versions %s and %s: %v", v1, v2, err)
				continue
			}
			
			results = append(results, *result)
		}
	}

	return results, nil
}

func (a *AnalysisAgent) compareVersions(version1, version2 string) (*types.AnalysisResult, error) {
	a.logger.Info("Comparing versions %s and %s", version1, version2)

	a.mu.RLock()
	files1 := a.parsedFiles[version1]
	files2 := a.parsedFiles[version2]
	a.mu.RUnlock()

	if files1 == nil || files2 == nil {
		return nil, fmt.Errorf("parsed files not available for comparison")
	}

	result := &types.AnalysisResult{
		Version1:                version1,
		Version2:                version2,
		StructDifferences:       make([]types.StructDiff, 0),
		FunctionDifferences:     make([]types.FunctionDiff, 0),
		ImportDifferences:       make([]types.ImportDiff, 0),
		ValidationDifferences:   make([]types.ValidationDiff, 0),
		BreakingChanges:         make([]types.BreakingChange, 0),
	}

	// Compare components_gen.go (main focus for struct differences)
	if ast1, ok := files1["components_gen.go"]; ok {
		if ast2, ok := files2["components_gen.go"]; ok {
			a.compareStructs(ast1, ast2, result)
			a.compareImports(ast1, ast2, result)
			a.compareValidations(ast1, ast2, result)
		}
	}

	// Compare routes_gen.go (focus on function signatures)
	if ast1, ok := files1["routes_gen.go"]; ok {
		if ast2, ok := files2["routes_gen.go"]; ok {
			a.compareFunctions(ast1, ast2, result)
		}
	}

	// Calculate compatibility score
	result.CompatibilityScore = a.calculateCompatibilityScore(result)
	result.Summary = a.generateSummary(result)

	a.logger.Info("Comparison completed: %s vs %s (score: %.2f)", version1, version2, result.CompatibilityScore)
	return result, nil
}

func (a *AnalysisAgent) compareStructs(ast1, ast2 *types.ASTInfo, result *types.AnalysisResult) {
	// Find structs that exist in both versions and compare them
	for name, struct1 := range ast1.Structs {
		if struct2, exists := ast2.Structs[name]; exists {
			diff := a.compareStruct(struct1, struct2)
			if diff != nil {
				result.StructDifferences = append(result.StructDifferences, *diff)
			}
		} else {
			// Struct removed in version2
			diff := types.StructDiff{
				Name:       name,
				ChangeType: types.ChangeRemoved,
				OldStruct:  struct1,
				Description: fmt.Sprintf("Struct %s removed in %s", name, result.Version2),
			}
			result.StructDifferences = append(result.StructDifferences, diff)
			
			// This is a breaking change
			result.BreakingChanges = append(result.BreakingChanges, types.BreakingChange{
				Type:        "struct_removed",
				Description: fmt.Sprintf("Struct %s was removed", name),
				Impact:      "Code using this struct will fail to compile",
				Severity:    "high",
			})
		}
	}

	// Find structs added in version2
	for name, struct2 := range ast2.Structs {
		if _, exists := ast1.Structs[name]; !exists {
			diff := types.StructDiff{
				Name:       name,
				ChangeType: types.ChangeAdded,
				NewStruct:  struct2,
				Description: fmt.Sprintf("Struct %s added in %s", name, result.Version2),
			}
			result.StructDifferences = append(result.StructDifferences, diff)
		}
	}
}

func (a *AnalysisAgent) compareStruct(struct1, struct2 *types.StructInfo) *types.StructDiff {
	var fieldDiffs []types.FieldDiff
	hasChanges := false

	// Compare fields
	for fieldName, fieldType1 := range struct1.Fields {
		if fieldType2, exists := struct2.Fields[fieldName]; exists {
			if fieldType1 != fieldType2 {
				fieldDiffs = append(fieldDiffs, types.FieldDiff{
					Name:       fieldName,
					ChangeType: types.ChangeModified,
					OldType:    fieldType1,
					NewType:    fieldType2,
					Impact:     "Type change may cause compilation errors",
				})
				hasChanges = true
			}
			
			// Compare tags
			tag1 := struct1.Tags[fieldName]
			tag2 := struct2.Tags[fieldName]
			if tag1 != tag2 {
				fieldDiffs = append(fieldDiffs, types.FieldDiff{
					Name:       fieldName,
					ChangeType: types.ChangeModified,
					OldTags:    tag1,
					NewTags:    tag2,
					Impact:     "Tag change may affect serialization/validation",
				})
				hasChanges = true
			}
		} else {
			// Field removed
			fieldDiffs = append(fieldDiffs, types.FieldDiff{
				Name:       fieldName,
				ChangeType: types.ChangeRemoved,
				OldType:    fieldType1,
				Impact:     "Removed field is a breaking change",
			})
			hasChanges = true
		}
	}

	// Find added fields
	for fieldName, fieldType2 := range struct2.Fields {
		if _, exists := struct1.Fields[fieldName]; !exists {
			fieldDiffs = append(fieldDiffs, types.FieldDiff{
				Name:       fieldName,
				ChangeType: types.ChangeAdded,
				NewType:    fieldType2,
				Impact:     "New field is generally backward compatible",
			})
			hasChanges = true
		}
	}

	if !hasChanges {
		return nil
	}

	return &types.StructDiff{
		Name:        struct1.Name,
		ChangeType:  types.ChangeModified,
		OldStruct:   struct1,
		NewStruct:   struct2,
		FieldDiffs:  fieldDiffs,
		Description: fmt.Sprintf("Struct %s has %d field differences", struct1.Name, len(fieldDiffs)),
	}
}

func (a *AnalysisAgent) compareImports(ast1, ast2 *types.ASTInfo, result *types.AnalysisResult) {
	imports1 := make(map[string]bool)
	imports2 := make(map[string]bool)
	
	for _, imp := range ast1.Imports {
		imports1[imp] = true
	}
	for _, imp := range ast2.Imports {
		imports2[imp] = true
	}

	// Find removed imports
	for imp := range imports1 {
		if !imports2[imp] {
			result.ImportDifferences = append(result.ImportDifferences, types.ImportDiff{
				Package:     imp,
				ChangeType:  types.ChangeRemoved,
				Description: fmt.Sprintf("Import %s removed in %s", imp, result.Version2),
			})
		}
	}

	// Find added imports
	for imp := range imports2 {
		if !imports1[imp] {
			result.ImportDifferences = append(result.ImportDifferences, types.ImportDiff{
				Package:     imp,
				ChangeType:  types.ChangeAdded,
				Description: fmt.Sprintf("Import %s added in %s", imp, result.Version2),
			})
		}
	}
}

func (a *AnalysisAgent) compareFunctions(ast1, ast2 *types.ASTInfo, result *types.AnalysisResult) {
	// Group functions by operation patterns (HTTP operations)
	ops1 := a.groupFunctionsByOperation(ast1.Functions)
	ops2 := a.groupFunctionsByOperation(ast2.Functions)

	// Compare operations (high-level changes)
	a.compareOperations(ops1, ops2, result)

	// Compare functions within same operations (detailed changes)
	for opName, functions1 := range ops1 {
		if functions2, exists := ops2[opName]; exists {
			a.compareFunctionsInOperation(opName, functions1, functions2, result)
		}
	}
}

// Group functions by HTTP operation (Post, Get, Put, Delete + endpoint)
func (a *AnalysisAgent) groupFunctionsByOperation(functions map[string]string) map[string]map[string]string {
	operations := make(map[string]map[string]string)
	
	for funcName, signature := range functions {
		opName := a.extractOperationName(funcName)
		if opName == "" {
			opName = "common" // Functions that don't belong to specific HTTP operations
		}
		
		if operations[opName] == nil {
			operations[opName] = make(map[string]string)
		}
		operations[opName][funcName] = signature
	}
	
	return operations
}

// Extract operation name from function name (e.g., "*PostTransaction*" -> "PostTransaction")
func (a *AnalysisAgent) extractOperationName(funcName string) string {
	// Common HTTP operation patterns in generated code
	patterns := []string{
		"PostTransaction", "PutTransaction", "DeleteTransactionsUUID",
		"GetSecureEndpoint", "GetSemiSecureEndpoint", "PostBearerEndpoint",
		"PostCallbacksCallbackType",
	}
	
	for _, pattern := range patterns {
		if strings.Contains(funcName, pattern) {
			return pattern
		}
	}
	
	return ""
}

// Compare high-level operations (added/removed endpoints)
func (a *AnalysisAgent) compareOperations(ops1, ops2 map[string]map[string]string, result *types.AnalysisResult) {
	// Find removed operations
	for opName := range ops1 {
		if opName == "common" {
			continue // Skip common functions
		}
		if _, exists := ops2[opName]; !exists {
			result.FunctionDifferences = append(result.FunctionDifferences, types.FunctionDiff{
				Name:        opName,
				ChangeType:  types.ChangeRemoved,
				Description: fmt.Sprintf("HTTP operation %s completely removed", opName),
			})
		}
	}
	
	// Find added operations
	for opName := range ops2 {
		if opName == "common" {
			continue // Skip common functions
		}
		if _, exists := ops1[opName]; !exists {
			result.FunctionDifferences = append(result.FunctionDifferences, types.FunctionDiff{
				Name:        opName,
				ChangeType:  types.ChangeAdded,
				Description: fmt.Sprintf("HTTP operation %s added", opName),
			})
		}
	}
}

// Compare functions within the same operation
func (a *AnalysisAgent) compareFunctionsInOperation(opName string, functions1, functions2 map[string]string, result *types.AnalysisResult) {
	// Only compare functions with identical full names (including receiver)
	exactMatches := 0
	
	for funcName, sig1 := range functions1 {
		if sig2, exists := functions2[funcName]; exists {
			exactMatches++
			if sig1 != sig2 {
				// Real signature change within same operation
				result.FunctionDifferences = append(result.FunctionDifferences, types.FunctionDiff{
					Name:         funcName,
					ChangeType:   types.ChangeModified,
					OldSignature: sig1,
					NewSignature: sig2,
					Description:  fmt.Sprintf("BREAKING: Function %s in operation %s signature changed", funcName, opName),
				})
				
				// Add as breaking change
				result.BreakingChanges = append(result.BreakingChanges, types.BreakingChange{
					Type:        "function_signature_changed",
					Description: fmt.Sprintf("Function %s signature changed", funcName),
					Impact:      "Code using this function will fail to compile",
					Severity:    "high",
				})
			}
		} else {
			// Function actually removed from this operation
			result.FunctionDifferences = append(result.FunctionDifferences, types.FunctionDiff{
				Name:         funcName,
				ChangeType:   types.ChangeRemoved,
				OldSignature: sig1,
				Description:  fmt.Sprintf("BREAKING: Function %s removed from operation %s", funcName, opName),
			})
			
			// Add as breaking change
			result.BreakingChanges = append(result.BreakingChanges, types.BreakingChange{
				Type:        "function_removed",
				Description: fmt.Sprintf("Function %s was removed", funcName),
				Impact:      "Code using this function will fail to compile",
				Severity:    "high",
			})
		}
	}

	// Find added functions in this operation
	for funcName, sig2 := range functions2 {
		if _, exists := functions1[funcName]; !exists {
			result.FunctionDifferences = append(result.FunctionDifferences, types.FunctionDiff{
				Name:         funcName,
				ChangeType:   types.ChangeAdded,
				NewSignature: sig2,
				Description:  fmt.Sprintf("Function %s added to operation %s", funcName, opName),
			})
		}
	}
	
	// Log debugging info for common functions only
	if opName == "common" && exactMatches > 0 {
		a.logger.Debug("Operation %s: matched %d common functions", opName, exactMatches)
	}
}

func (a *AnalysisAgent) compareValidations(ast1, ast2 *types.ASTInfo, result *types.AnalysisResult) {
	// This is a simplified validation comparison
	// In a real implementation, we would parse validation tags and rules more thoroughly
	for name, struct1 := range ast1.Structs {
		if struct2, exists := ast2.Structs[name]; exists {
			for fieldName, tag1 := range struct1.Tags {
				if tag2, exists := struct2.Tags[fieldName]; exists && tag1 != tag2 {
					result.ValidationDifferences = append(result.ValidationDifferences, types.ValidationDiff{
						Field:       fmt.Sprintf("%s.%s", name, fieldName),
						ChangeType:  types.ChangeModified,
						OldRule:     tag1,
						NewRule:     tag2,
						Impact:      "Validation rule change may affect data validation",
						Description: fmt.Sprintf("Validation rule changed for %s.%s", name, fieldName),
					})
				}
			}
		}
	}
}

func (a *AnalysisAgent) calculateCompatibilityScore(result *types.AnalysisResult) float64 {
	totalChanges := len(result.StructDifferences) + 
		          len(result.FunctionDifferences) + 
		          len(result.ImportDifferences) + 
		          len(result.ValidationDifferences)
	
	breakingChanges := len(result.BreakingChanges)
	
	if totalChanges == 0 {
		return 100.0
	}
	
	// Calculate score based on ratio of breaking vs total changes
	compatibilityRatio := float64(totalChanges-breakingChanges) / float64(totalChanges)
	return compatibilityRatio * 100.0
}

func (a *AnalysisAgent) generateSummary(result *types.AnalysisResult) string {
	summary := fmt.Sprintf("Comparison between %s and %s: ", result.Version1, result.Version2)
	
	totalChanges := len(result.StructDifferences) + len(result.FunctionDifferences) + 
		          len(result.ImportDifferences) + len(result.ValidationDifferences)
	
	if totalChanges == 0 {
		summary += "No differences found - fully compatible"
	} else {
		summary += fmt.Sprintf("%d total differences, %d breaking changes, %.1f%% compatibility", 
			totalChanges, len(result.BreakingChanges), result.CompatibilityScore)
	}
	
	return summary
}

func (a *AnalysisAgent) GetAnalysisResults() []types.AnalysisResult {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	results := make([]types.AnalysisResult, len(a.analysisResults))
	copy(results, a.analysisResults)
	return results
}

func (a *AnalysisAgent) GetParsedFiles() map[string]map[string]*types.ASTInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	result := make(map[string]map[string]*types.ASTInfo)
	for version, files := range a.parsedFiles {
		result[version] = make(map[string]*types.ASTInfo)
		for filename, astInfo := range files {
			result[version][filename] = astInfo
		}
	}
	return result
}