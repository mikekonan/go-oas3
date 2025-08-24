package errors

import (
	"fmt"
	"strings"
	"time"
)

// ValidationError represents errors that occur during the validation process
type ValidationError struct {
	Phase     string    `json:"phase"`
	Agent     string    `json:"agent"`
	Message   string    `json:"message"`
	Cause     error     `json:"-"`
	Timestamp time.Time `json:"timestamp"`
	Severity  Severity  `json:"severity"`
	Retryable bool      `json:"retryable"`
}

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityCritical Severity = "critical"
)

func (e *ValidationError) Error() string {
	base := fmt.Sprintf("[%s/%s] %s", e.Phase, e.Agent, e.Message)
	if e.Cause != nil {
		base += fmt.Sprintf(": %v", e.Cause)
	}
	return base
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

func NewValidationError(phase, agent, message string, cause error) *ValidationError {
	return &ValidationError{
		Phase:     phase,
		Agent:     agent,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Severity:  SeverityError,
		Retryable: false,
	}
}

func NewRetryableError(phase, agent, message string, cause error) *ValidationError {
	return &ValidationError{
		Phase:     phase,
		Agent:     agent,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Severity:  SeverityWarning,
		Retryable: true,
	}
}

func NewCriticalError(phase, agent, message string, cause error) *ValidationError {
	return &ValidationError{
		Phase:     phase,
		Agent:     agent,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Severity:  SeverityCritical,
		Retryable: false,
	}
}

// ErrorCollector collects and manages errors during validation
type ErrorCollector struct {
	errors   []*ValidationError
	warnings []*ValidationError
}

func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors:   make([]*ValidationError, 0),
		warnings: make([]*ValidationError, 0),
	}
}

func (ec *ErrorCollector) AddError(err *ValidationError) {
	if err.Severity == SeverityWarning || err.Severity == SeverityInfo {
		ec.warnings = append(ec.warnings, err)
	} else {
		ec.errors = append(ec.errors, err)
	}
}

func (ec *ErrorCollector) AddValidationError(phase, agent, message string, cause error) {
	ec.AddError(NewValidationError(phase, agent, message, cause))
}

func (ec *ErrorCollector) AddWarning(phase, agent, message string, cause error) {
	warning := NewValidationError(phase, agent, message, cause)
	warning.Severity = SeverityWarning
	ec.AddError(warning)
}

func (ec *ErrorCollector) HasErrors() bool {
	return len(ec.errors) > 0
}

func (ec *ErrorCollector) HasWarnings() bool {
	return len(ec.warnings) > 0
}

func (ec *ErrorCollector) GetErrors() []*ValidationError {
	return ec.errors
}

func (ec *ErrorCollector) GetWarnings() []*ValidationError {
	return ec.warnings
}

func (ec *ErrorCollector) GetAllErrors() []*ValidationError {
	all := make([]*ValidationError, 0, len(ec.errors)+len(ec.warnings))
	all = append(all, ec.errors...)
	all = append(all, ec.warnings...)
	return all
}

func (ec *ErrorCollector) Clear() {
	ec.errors = ec.errors[:0]
	ec.warnings = ec.warnings[:0]
}

func (ec *ErrorCollector) Summary() string {
	if !ec.HasErrors() && !ec.HasWarnings() {
		return "No errors or warnings"
	}

	parts := []string{}
	if ec.HasErrors() {
		parts = append(parts, fmt.Sprintf("%d errors", len(ec.errors)))
	}
	if ec.HasWarnings() {
		parts = append(parts, fmt.Sprintf("%d warnings", len(ec.warnings)))
	}

	return strings.Join(parts, ", ")
}

func (ec *ErrorCollector) DetailedSummary() string {
	if !ec.HasErrors() && !ec.HasWarnings() {
		return "No errors or warnings"
	}

	var builder strings.Builder
	
	if ec.HasErrors() {
		builder.WriteString(fmt.Sprintf("Errors (%d):\n", len(ec.errors)))
		for i, err := range ec.errors {
			builder.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
		}
		builder.WriteString("\n")
	}

	if ec.HasWarnings() {
		builder.WriteString(fmt.Sprintf("Warnings (%d):\n", len(ec.warnings)))
		for i, warn := range ec.warnings {
			builder.WriteString(fmt.Sprintf("  %d. %s\n", i+1, warn.Error()))
		}
	}

	return builder.String()
}

// RetryableRunner handles retrying operations with backoff
type RetryableRunner struct {
	MaxRetries int
	BaseDelay  time.Duration
}

func NewRetryableRunner() *RetryableRunner {
	return &RetryableRunner{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
	}
}

func (r *RetryableRunner) Run(operation func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * r.BaseDelay
			time.Sleep(delay)
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		
		// Check if error is retryable
		if validationErr, ok := err.(*ValidationError); ok && !validationErr.Retryable {
			return err
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", r.MaxRetries, lastErr)
}

// ValidationGate represents a validation checkpoint
type ValidationGate struct {
	Name        string
	Description string
	Required    bool
	Validator   func() error
}

func (vg *ValidationGate) Execute() error {
	if vg.Validator == nil {
		return fmt.Errorf("no validator function provided for gate %s", vg.Name)
	}
	
	if err := vg.Validator(); err != nil {
		if vg.Required {
			return NewCriticalError("validation-gate", vg.Name, 
				fmt.Sprintf("Required validation gate failed: %s", vg.Description), err)
		} else {
			return NewValidationError("validation-gate", vg.Name, 
				fmt.Sprintf("Optional validation gate failed: %s", vg.Description), err)
		}
	}
	
	return nil
}

// GateRunner manages and executes validation gates
type GateRunner struct {
	gates []ValidationGate
	collector *ErrorCollector
}

func NewGateRunner() *GateRunner {
	return &GateRunner{
		gates:     make([]ValidationGate, 0),
		collector: NewErrorCollector(),
	}
}

func (gr *GateRunner) AddGate(gate ValidationGate) {
	gr.gates = append(gr.gates, gate)
}

func (gr *GateRunner) ExecuteAll() error {
	for _, gate := range gr.gates {
		if err := gate.Execute(); err != nil {
			if validationErr, ok := err.(*ValidationError); ok {
				gr.collector.AddError(validationErr)
			} else {
				gr.collector.AddValidationError("validation-gate", gate.Name, "Gate execution failed", err)
			}

			// Stop on critical errors
			if validationErr, ok := err.(*ValidationError); ok && validationErr.Severity == SeverityCritical {
				return fmt.Errorf("critical validation gate failure: %w", err)
			}
		}
	}

	if gr.collector.HasErrors() {
		return fmt.Errorf("validation gates failed: %s", gr.collector.Summary())
	}

	return nil
}

func (gr *GateRunner) GetErrorCollector() *ErrorCollector {
	return gr.collector
}