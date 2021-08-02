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

package cargo

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
)

// CLIRunner can execute cargo via CLI
type CLIRunner struct {
	ConfigResolver libpak.ConfigurationResolver
	Executor       effect.Executor
	Logger         bard.Logger
}

// NewCLIRunner creates a new Cargo Runner using the cargo cli
func NewCLIRunner(configResolver libpak.ConfigurationResolver, executor effect.Executor, logger bard.Logger) CLIRunner {
	return CLIRunner{
		ConfigResolver: configResolver,
		Executor:       executor,
		Logger:         logger,
	}
}

// Install will build and install the project using `cargo install`
func (c CLIRunner) Install(srcDir string, destLayer libcnb.Layer) error {
	return c.InstallMember(".", srcDir, destLayer)
}

// InstallMember will build and install a specific workspace member using `cargo install`
func (c CLIRunner) InstallMember(memberPath string, srcDir string, destLayer libcnb.Layer) error {
	args, err := c.BuildArgs(destLayer, memberPath)
	if err != nil {
		return err
	}

	c.Logger.Bodyf("cargo %s", strings.Join(args, " "))
	if err := c.Executor.Execute(effect.Execution{
		Command: "cargo",
		Args:    args,
		Dir:     srcDir,
		Stdout:  c.Logger.InfoWriter(),
		Stderr:  c.Logger.InfoWriter(),
	}); err != nil {
		return fmt.Errorf("unable to build\n%w", err)
	}

	err = c.CleanCargoHomeCache()
	if err != nil {
		return fmt.Errorf("unable to cleanup: %w", err)
	}
	return nil
}

type metadata struct {
	WorkspaceMembers []string `json:"workspace_members"`
}

// WorkspaceMembers loads the members from the project workspace
func (c CLIRunner) WorkspaceMembers(srcDir string, destLayer libcnb.Layer) ([]url.URL, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	if err := c.Executor.Execute(effect.Execution{
		Command: "cargo",
		Args:    []string{"metadata", "--format-version=1", "--no-deps"},
		Dir:     srcDir,
		Stdout:  &stdout,
		Stderr:  &stderr,
	}); err != nil {
		return nil, fmt.Errorf("unable to read metadata: \n%s\n%s\n%w", &stdout, &stderr, err)
	}

	var m metadata
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		return nil, fmt.Errorf("unable to parse Cargo metadata: %w", err)
	}

	filterStr, filter := c.ConfigResolver.Resolve("BP_CARGO_WORKSPACE_MEMBERS")
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

func (c CLIRunner) CleanCargoHomeCache() error {
	cargoHome, found := c.ConfigResolver.Resolve("CARGO_HOME")
	if !found || strings.TrimSpace(cargoHome) == "" {
		return fmt.Errorf("unable to find CARGO_HOME")
	}

	files, err := os.ReadDir(cargoHome)
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
		err := os.RemoveAll(filepath.Join(cargoHome, file.Name()))
		if err != nil {
			return fmt.Errorf("unable to remove files\n%w", err)
		}
	}

	registryDir := filepath.Join(cargoHome, "registry")
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

	gitDir := filepath.Join(cargoHome, "git")
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

func (c CLIRunner) CargoVersion() (string, error) {
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

func (c CLIRunner) RustVersion() (string, error) {
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
func (c CLIRunner) BuildArgs(destLayer libcnb.Layer, defaultMemberPath string) ([]string, error) {
	rawArgs, _ := c.ConfigResolver.Resolve("BP_CARGO_INSTALL_ARGS")

	envArgs, err := FilterInstallArgs(rawArgs)
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

func (c CLIRunner) AsBOMEntry() (libcnb.BOMEntry, error) {
	// TODO: read through cargo manifest and dump dependencies
	//   libbs is using `libjvm.NewMavenJARListing(c.Path)`

	return libcnb.BOMEntry{
		Name:     "build-dependencies",
		Metadata: map[string]interface{}{"dependencies": "TODO"},
		Build:    true,
	}, nil
}
