package cargo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-shellwords"
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

	args, err := c.BuildArgs(destLayer)
	if err != nil {
		return err
	}

	err = c.exec.Execute(pexec.Execution{
		Dir:    srcDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    env,
		Args:   args,
	})
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}

// BuildArgs will build the list of arguments to pass `cargo install`
func (c CLIRunner) BuildArgs(destLayer packit.Layer) ([]string, error) {
	env_args, err := c.FilterInstallArgs(os.Getenv("BP_CARGO_INSTALL_ARGS"))
	if err != nil {
		return nil, fmt.Errorf("filter failed: %w", err)
	}

	args := []string{"install"}
	args = append(args, env_args...)
	args = append(args, "--color=never", fmt.Sprintf("--root=%s", destLayer.Path))
	args = c.AddDefaultPath(args)

	return args, nil
}

// FilterInstallArg provides a clean list of allowed arguments
func (c CLIRunner) FilterInstallArgs(args string) ([]string, error) {
	argwords, err := shellwords.Parse(args)
	if err != nil {
		return nil, fmt.Errorf("parse args failed: %w", err)
	}

	var filteredArgs []string
	skipNext := false
	for _, arg := range argwords {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--root" || arg == "--color" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(arg, "--root=") || strings.HasPrefix(arg, "--color=") {
			continue
		}
		filteredArgs = append(filteredArgs, arg)
	}

	return filteredArgs, nil
}

// AddDefaultPath will add --path=. if --path is not set
func (c CLIRunner) AddDefaultPath(args []string) []string {
	for _, arg := range args {
		if arg == "--path" || strings.HasPrefix(arg, "--path=") {
			return args
		}
	}
	return append(args, "--path=.")
}
