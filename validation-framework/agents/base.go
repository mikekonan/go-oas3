package agents

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mikekonan/go-oas3/validation-framework/types"
)

type BaseAgent struct {
	id       string
	name     string
	config   *types.ValidationConfig
	logger   types.Logger
	workDir  string
}

func NewBaseAgent(id, name string, config *types.ValidationConfig) *BaseAgent {
	return &BaseAgent{
		id:      id,
		name:    name,
		config:  config,
		logger:  &DefaultLogger{},
		workDir: config.WorkspaceDir,
	}
}

func (b *BaseAgent) ID() string {
	return b.id
}

func (b *BaseAgent) Name() string {
	return b.name
}

func (b *BaseAgent) Config() *types.ValidationConfig {
	return b.config
}

func (b *BaseAgent) Logger() types.Logger {
	return b.logger
}

func (b *BaseAgent) SetLogger(logger types.Logger) {
	b.logger = logger
}

func (b *BaseAgent) WorkDir() string {
	return b.workDir
}

func (b *BaseAgent) EnsureDir(dir string) error {
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(b.workDir, dir)
	}
	return os.MkdirAll(dir, 0755)
}

func (b *BaseAgent) Validate() error {
	if b.config == nil {
		return fmt.Errorf("config is required")
	}
	if b.workDir == "" {
		return fmt.Errorf("work directory is required")
	}
	return nil
}

type DefaultLogger struct{}

func (d *DefaultLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (d *DefaultLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

func (d *DefaultLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] "+msg, args...)
}

func (d *DefaultLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}