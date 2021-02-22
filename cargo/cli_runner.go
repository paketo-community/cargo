package cargo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/pexec"
)

//go:generate mockery --name Executable --case=underscore

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

// Install will build and install a project using `cargo install`
func (c CLIRunner) Install(srcDir string, workLayer packit.Layer, destLayer packit.Layer) error {
	env := os.Environ()
	env = append(env, fmt.Sprintf("CARGO_TARGET_DIR=%s", workLayer.Path))

	for i := 0; i < len(env); i++ {
		if strings.HasPrefix(env[i], "PATH=") {
			env[i] = fmt.Sprintf("%s%c%s", env[i], os.PathListSeparator, filepath.Join(destLayer.Path, "bin"))
		}
	}

	err := c.exec.Execute(pexec.Execution{
		Dir:    srcDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    env,
		Args: []string{
			"install",
			"--color=never",
			"--path=.",
			fmt.Sprintf("--root=%s", destLayer.Path),
		},
		// TODO: look at adding --frozen or --locked
		// TODO: --offline, maybe?
		// TODO: pull in extra args from an env variable
	})
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}
