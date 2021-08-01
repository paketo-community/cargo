package cargo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mattn/go-shellwords"
	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/scribe"
)

//go:generate mockery --name Executable --case=underscore

// Executable allows for mocking the pexec.Executable
type Executable interface {
	Execute(execution pexec.Execution) error
}

// CLIRunner can execute cargo via CLI
type CLIRunner struct {
	exec   Executable
	logger scribe.Emitter
}

// NewCLIRunner creates a new Cargo Runner using the cargo cli
func NewCLIRunner(exec Executable, logger scribe.Emitter) CLIRunner {
	return CLIRunner{
		exec:   exec,
		logger: logger,
	}
}

func createEnviron(workLayer packit.Layer, destLayer packit.Layer) []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("CARGO_TARGET_DIR=%s", path.Join(workLayer.Path, "target")))
	env = append(env, fmt.Sprintf("CARGO_HOME=%s", path.Join(workLayer.Path, "home")))

	for i := 0; i < len(env); i++ {
		if strings.HasPrefix(env[i], "PATH=") {
			env[i] = fmt.Sprintf("%s%c%s", env[i], os.PathListSeparator, filepath.Join(destLayer.Path, "bin"))
		}
	}

	return env
}

// Install will build and install the project using `cargo install`
func (c CLIRunner) Install(srcDir string, workLayer packit.Layer, destLayer packit.Layer) error {
	return c.InstallMember(".", srcDir, workLayer, destLayer)
}

// InstallMember will build and install a specific workspace member using `cargo install`
func (c CLIRunner) InstallMember(memberPath string, srcDir string, workLayer packit.Layer, destLayer packit.Layer) error {
	args, err := c.BuildArgs(destLayer, memberPath)
	if err != nil {
		return err
	}

	c.logger.Detail("cargo %s", strings.Join(args, " "))
	err = c.exec.Execute(pexec.Execution{
		Dir:    srcDir,
		Stdout: scribe.NewWriter(os.Stdout, scribe.WithIndent(5)),
		Stderr: scribe.NewWriter(os.Stderr, scribe.WithIndent(5)),
		Env:    createEnviron(workLayer, destLayer),
		Args:   args,
	})
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	err = c.CleanCargoHomeCache(workLayer)
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	return nil
}

type metadata struct {
	WorkspaceMembers []string `json:"workspace_members"`
}

// WorkspaceMembers loads the members from the project workspace
func (c CLIRunner) WorkspaceMembers(srcDir string, workLayer packit.Layer, destLayer packit.Layer) ([]url.URL, error) {
	stdout := bytes.Buffer{}

	err := c.exec.Execute(pexec.Execution{
		Dir:    srcDir,
		Stdout: &stdout,
		Env:    createEnviron(workLayer, destLayer),
		Args:   []string{"metadata", "--format-version=1", "--no-deps"},
	})
	if err != nil {
		return nil, fmt.Errorf("build failed: %s\n%w", &stdout, err)
	}

	var m metadata
	err = json.Unmarshal(stdout.Bytes(), &m)
	if err != nil {
		return nil, fmt.Errorf("unable to parse Cargo metadata: %w", err)
	}

	filterStr, filter := os.LookupEnv("BP_CARGO_WORKSPACE_MEMBERS")
	filterList := make(map[string]bool)
	if filter {
		for _, f := range strings.Split(filterStr, ",") {
			filterList[strings.TrimSpace(f)] = true
		}
	}

	var paths []url.URL
	for _, workspace := range m.WorkspaceMembers {
		// This is OK because the workspace member format is `package-name package-version (url)` and
		//   none of name, version or URL may contain a space & be valid
		parts := strings.SplitN(workspace, " ", 3)
		if filter && filterList[strings.TrimSpace(parts[0])] || !filter {
			path, err := url.Parse(strings.TrimSuffix(strings.TrimPrefix(parts[2], "("), ")"))
			if err != nil {
				return nil, fmt.Errorf("unable to parse URL %s: %w", workspace, err)
			}
			paths = append(paths, *path)
		}
	}

	return paths, nil
}

func (c CLIRunner) CleanCargoHomeCache(workLayer packit.Layer) error {
	homeDir := filepath.Join(workLayer.Path, "home")
	files, err := os.ReadDir(homeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("unable to read directory\n%w", err)
	}

	for _, file := range files {
		if file.IsDir() && file.Name() == "bin" ||
			file.IsDir() && file.Name() == "registry" ||
			file.IsDir() && file.Name() == "git" {
			continue
		}
		err := os.RemoveAll(filepath.Join(homeDir, file.Name()))
		if err != nil {
			return fmt.Errorf("unable to remove files\n%w", err)
		}
	}

	registryDir := filepath.Join(homeDir, "registry")
	files, err = os.ReadDir(registryDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to read directory\n%w", err)
	}

	for _, file := range files {
		if file.IsDir() && file.Name() == "index" ||
			file.IsDir() && file.Name() == "cache" {
			continue
		}
		err := os.RemoveAll(filepath.Join(registryDir, file.Name()))
		if err != nil {
			return fmt.Errorf("unable to remove files\n%w", err)
		}
	}

	gitDir := filepath.Join(homeDir, "git")
	files, err = os.ReadDir(gitDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to read directory\n%w", err)
	}

	for _, file := range files {
		if file.IsDir() && file.Name() == "db" {
			continue
		}
		err := os.RemoveAll(filepath.Join(gitDir, file.Name()))
		if err != nil {
			return fmt.Errorf("unable to remove files\n%w", err)
		}
	}

	return nil
}

// BuildArgs will build the list of arguments to pass `cargo install`
func (c CLIRunner) BuildArgs(destLayer packit.Layer, defaultMemberPath string) ([]string, error) {
	envArgs, err := FilterInstallArgs(os.Getenv("BP_CARGO_INSTALL_ARGS"))
	if err != nil {
		return nil, fmt.Errorf("filter failed: %w", err)
	}

	args := []string{"install"}
	args = append(args, envArgs...)
	args = append(args, "--color=never", fmt.Sprintf("--root=%s", destLayer.Path))
	args = AddDefaultPath(args, defaultMemberPath)

	return args, nil
}

// FilterInstallArgs provides a clean list of allowed arguments
func FilterInstallArgs(args string) ([]string, error) {
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
func AddDefaultPath(args []string, defaultMemberPath string) []string {
	for _, arg := range args {
		if arg == "--path" || strings.HasPrefix(arg, "--path=") {
			return args
		}
	}
	return append(args, fmt.Sprintf("--path=%s", defaultMemberPath))
}
