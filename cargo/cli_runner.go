package cargo

import (
	"fmt"
	"os"

	"github.com/paketo-buildpacks/packit/pexec"
)

//go:generate mockery -name Executable -case=underscore

// Executable allows for mocking the pexec.Executable
type Executable interface {
	Execute(execution pexec.Execution) error
}

// CLIRunner can execute cargo via CLI
type CLIRunner struct {
	exec Executable
}

// NewCLIRunner creates a new Cargo Runner using the cargo cli
func NewCLIRunner(exec Executable) CLIRunner {
	return CLIRunner{
		exec: exec,
	}
}

// Build a project using `cargo build`
func (c CLIRunner) Build(workDir string) error {
	err := c.exec.Execute(pexec.Execution{
		Dir:    workDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Args:   []string{},
	})
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}
