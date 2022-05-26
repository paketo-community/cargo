/*
 * Copyright 2018-2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/buildpacks/libcnb"
	"github.com/mattn/go-shellwords"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sherpa"
)

//go:generate mockery --name CargoService --case underscore

type CargoService interface {
	Install(srcDir string, destLayer libcnb.Layer) error
	InstallMember(memberPath string, srcDir string, destLayer libcnb.Layer) error
	InstallTool(name string, additionalArgs []string) error
	WorkspaceMembers(srcDir string, destLayer libcnb.Layer) ([]url.URL, error)
	ProjectTargets(srcDir string) ([]string, error)
	CleanCargoHomeCache() error
	CargoVersion() (string, error)
	RustVersion() (string, error)
}

// Option is a function for configuring a CargoRunner
type Option func(runner CargoRunner) CargoRunner

// WithCargoHome sets CARGO_HOME
func WithCargoHome(cargoHome string) Option {
	return func(runner CargoRunner) CargoRunner {
		runner.CargoHome = cargoHome
		return runner
	}
}

// WithCargoWorkspaceMembers sets a comma separate list of workspace members
func WithCargoWorkspaceMembers(cargoWorkspaceMembers string) Option {
	return func(runner CargoRunner) CargoRunner {
		runner.CargoWorkspaceMembers = cargoWorkspaceMembers
		return runner
	}
}

// WithCargoInstallArgs sets addition args to pass to cargo install
func WithCargoInstallArgs(installArgs string) Option {
	return func(runner CargoRunner) CargoRunner {
		runner.CargoInstallArgs = installArgs
		return runner
	}
}

// WithExecutor sets the executor to use when running cargo
func WithExecutor(executor effect.Executor) Option {
	return func(runner CargoRunner) CargoRunner {
		runner.Executor = executor
		return runner
	}
}

// WithLogger sets additional args to pass to cargo install
func WithLogger(logger bard.Logger) Option {
	return func(runner CargoRunner) CargoRunner {
		runner.Logger = logger
		return runner
	}
}

// WithStack sets the stack on which we're running
func WithStack(stack string) Option {
	return func(runner CargoRunner) CargoRunner {
		runner.Stack = stack
		return runner
	}
}

// CargoRunner can execute cargo via CLI
type CargoRunner struct {
	CargoHome             string
	CargoWorkspaceMembers string
	CargoInstallArgs      string
	Executor              effect.Executor
	Logger                bard.Logger
	Stack                 string
}

type metadataTarget struct {
	Kind       []string `json:"kind"`
	CrateTypes []string `json:"crate_types"`
	Name       string   `json:"name"`
	SrcPath    string   `json:"src_path"`
	Edition    string   `json:"edition"`
	Doc        bool     `json:"doc"`
	Doctest    bool     `json:"doctest"`
	Test       bool     `json:"test"`
}

type metadataPackage struct {
	ID      string
	Targets []metadataTarget `json:"targets"`
}

type metadata struct {
	Packages         []metadataPackage `json:"packages"`
	WorkspaceMembers []string          `json:"workspace_members"`
}

// NewCargoRunner creates a new cargo runner with the given options
func NewCargoRunner(options ...Option) CargoRunner {
	runner := CargoRunner{}

	for _, option := range options {
		runner = option(runner)
	}

	return runner
}

// Install will build and install the project using `cargo install`
func (c CargoRunner) Install(srcDir string, destLayer libcnb.Layer) error {
	return c.InstallMember(".", srcDir, destLayer)
}

// InstallMember will build and install a specific workspace member using `cargo install`
func (c CargoRunner) InstallMember(memberPath string, srcDir string, destLayer libcnb.Layer) error {
	// makes warning from `cargo install` go away
	path := os.Getenv("PATH")
	if path != "" && !strings.Contains(path, destLayer.Path) {
		path = sherpa.AppendToEnvVar("PATH", ":", filepath.Join(destLayer.Path, "bin"))
		err := os.Setenv("PATH", path)
		if err != nil {
			return fmt.Errorf("unable to update PATH\n%w", err)
		}
	}

	args, err := c.BuildArgs(destLayer, memberPath)
	if err != nil {
		return fmt.Errorf("unable to build args\n%w", err)
	}

	c.Logger.Bodyf("cargo %s", strings.Join(args, " "))
	if err := c.Executor.Execute(effect.Execution{
		Command: "cargo",
		Args:    args,
		Dir:     srcDir,
		Stdout:  bard.NewWriter(c.Logger.Logger.InfoWriter(), bard.WithIndent(3)),
		Stderr:  bard.NewWriter(c.Logger.Logger.InfoWriter(), bard.WithIndent(3)),
	}); err != nil {
		return fmt.Errorf("unable to build\n%w", err)
	}

	err = c.CleanCargoHomeCache()
	if err != nil {
		return fmt.Errorf("unable to cleanup: %w", err)
	}
	return nil
}

func (c CargoRunner) InstallTool(name string, additionalArgs []string) error {
	args := []string{"install", name}
	args = append(args, additionalArgs...)

	c.Logger.Bodyf("cargo %s", strings.Join(args, " "))
	if err := c.Executor.Execute(effect.Execution{
		Command: "cargo",
		Args:    args,
		Stdout:  bard.NewWriter(c.Logger.Logger.InfoWriter(), bard.WithIndent(3)),
		Stderr:  bard.NewWriter(c.Logger.Logger.InfoWriter(), bard.WithIndent(3)),
	}); err != nil {
		return fmt.Errorf("unable to install tool\n%w", err)
	}

	return nil
}

// WorkspaceMembers loads the members from the project workspace
func (c CargoRunner) WorkspaceMembers(srcDir string, destLayer libcnb.Layer) ([]url.URL, error) {
	m, err := c.fetchCargoMetadata(srcDir)
	if err != nil {
		return []url.URL{}, fmt.Errorf("unable to load cargo metadata\n%w", err)
	}

	filterMap := c.makeFilterMap()

	var paths []url.URL
	for _, workspace := range m.WorkspaceMembers {
		// This is OK because the workspace member format is `package-name package-version (url)` and
		//   none of name, version or URL may contain a space & be valid
		parts := strings.SplitN(workspace, " ", 3)
		if len(filterMap) > 0 && filterMap[strings.TrimSpace(parts[0])] || len(filterMap) == 0 {
			path, err := url.Parse(strings.TrimSuffix(strings.TrimPrefix(parts[2], "("), ")"))
			if err != nil {
				return nil, fmt.Errorf("unable to parse URL %s: %w", workspace, err)
			}
			paths = append(paths, *path)
		}
	}

	return paths, nil
}

// ProjectTargets loads the members from the project workspace
func (c CargoRunner) ProjectTargets(srcDir string) ([]string, error) {
	m, err := c.fetchCargoMetadata(srcDir)
	if err != nil {
		return []string{}, fmt.Errorf("unable to load cargo metadata\n%w", err)
	}

	filterMap := c.makeFilterMap()

	workspaces := []string{}
	for _, workspace := range m.WorkspaceMembers {
		// This is OK because the workspace member format is `package-name package-version (url)` and
		//   none of name, version or URL may contain a space & be valid
		parts := strings.SplitN(workspace, " ", 3)
		if len(filterMap) > 0 && filterMap[strings.TrimSpace(parts[0])] || len(filterMap) == 0 {
			workspaces = append(workspaces, workspace)
		}
	}

	var names []string
	for _, pkg := range m.Packages {
		for _, workspace := range workspaces {
			if pkg.ID == workspace {
				for _, target := range pkg.Targets {
					for _, kind := range target.Kind {
						if kind == "bin" && strings.HasPrefix(target.SrcPath, srcDir) {
							names = append(names, target.Name)
						}
					}
				}
			}
		}
	}

	return names, nil
}

// CleanCargoHomeCache clears out unnecessary files from under $CARGO_HOME
func (c CargoRunner) CleanCargoHomeCache() error {
	files, err := os.ReadDir(c.CargoHome)
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
		err := os.RemoveAll(filepath.Join(c.CargoHome, file.Name()))
		if err != nil {
			return fmt.Errorf("unable to remove files\n%w", err)
		}
	}

	registryDir := filepath.Join(c.CargoHome, "registry")
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

	gitDir := filepath.Join(c.CargoHome, "git")
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

// CargoVersion returns the version of cargo installed
func (c CargoRunner) CargoVersion() (string, error) {
	buf := &bytes.Buffer{}

	if err := c.Executor.Execute(effect.Execution{
		Command: "cargo",
		Args:    []string{"version"},
		Stdout:  buf,
		Stderr:  buf,
	}); err != nil {
		return "", fmt.Errorf("error executing 'cargo version':\n Combined Output: %s: \n%w", buf.String(), err)
	}

	s := strings.SplitN(strings.TrimSpace(buf.String()), " ", 3)
	return s[1], nil
}

// RustVersion returns the version of rustc installed
func (c CargoRunner) RustVersion() (string, error) {
	buf := &bytes.Buffer{}

	if err := c.Executor.Execute(effect.Execution{
		Command: "rustc",
		Args:    []string{"--version"},
		Stdout:  buf,
		Stderr:  buf,
	}); err != nil {
		return "", fmt.Errorf("error executing 'rustc --version':\n Combined Output: %s: \n%w", buf.String(), err)
	}

	s := strings.Split(strings.TrimSpace(buf.String()), " ")
	return s[1], nil
}

// BuildArgs will build the list of arguments to pass `cargo install`
func (c CargoRunner) BuildArgs(destLayer libcnb.Layer, defaultMemberPath string) ([]string, error) {
	envArgs, err := FilterInstallArgs(c.CargoInstallArgs)
	if err != nil {
		return nil, fmt.Errorf("filter failed: %w", err)
	}

	args := []string{"install"}
	args = append(args, envArgs...)
	args = append(args, "--color=never", fmt.Sprintf("--root=%s", destLayer.Path))
	args = AddDefaultPath(args, defaultMemberPath)
	args = AddDefaultTargetForTiny(args, c.Stack)

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

// AddDefaultTargetForTiny will add --target=x86_64-unknown-linux-musl if the stack is Tiny and a `--target` is not already set
func AddDefaultTargetForTiny(args []string, stack string) []string {
	if stack != libpak.TinyStackID {
		return args
	}

	for _, arg := range args {
		if arg == "--target" || strings.HasPrefix(arg, "--target=") {
			return args
		}
	}

	return append(args, "--target=x86_64-unknown-linux-musl")
}

func (c CargoRunner) fetchCargoMetadata(srcDir string) (metadata, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	if err := c.Executor.Execute(effect.Execution{
		Command: "cargo",
		Args:    []string{"metadata", "--format-version=1", "--no-deps"},
		Dir:     srcDir,
		Stdout:  &stdout,
		Stderr:  &stderr,
	}); err != nil {
		return metadata{}, fmt.Errorf("unable to read metadata: \n%s\n%s\n%w", &stdout, &stderr, err)
	}

	var m metadata
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		return metadata{}, fmt.Errorf("unable to parse Cargo metadata: %w", err)
	}

	return m, nil
}

func (c CargoRunner) makeFilterMap() map[string]bool {
	filter := c.CargoWorkspaceMembers != ""
	filterMap := make(map[string]bool)
	if filter {
		if !strings.Contains(c.CargoWorkspaceMembers, ",") {
			filterMap[c.CargoWorkspaceMembers] = true
		}
		for _, f := range strings.Split(c.CargoWorkspaceMembers, ",") {
			filterMap[strings.TrimSpace(f)] = true
		}
	}

	return filterMap
}
