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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/sbom"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"github.com/paketo-community/cargo/mtimes"
	"github.com/paketo-community/cargo/runner"
)

// Option is a function for configuring a Cargo
type Option func(cargo Cargo) Cargo

// WithApplicationPath sets app path
func WithApplicationPath(ap string) Option {
	return func(cargo Cargo) Cargo {
		cargo.ApplicationPath = ap
		return cargo
	}
}

// WithWorkspaceMembers sets workspace members
func WithWorkspaceMembers(ap string) Option {
	return func(cargo Cargo) Cargo {
		cargo.WorkspaceMembers = ap
		return cargo
	}
}

// WithRunSBOMScan sets workspace members
func WithRunSBOMScan(sc bool) Option {
	return func(cargo Cargo) Cargo {
		cargo.RunSBOMScan = sc
		return cargo
	}
}

// WithSBOMScanner sets workspace members
func WithSBOMScanner(sc sbom.SBOMScanner) Option {
	return func(cargo Cargo) Cargo {
		cargo.SBOMScanner = sc
		return cargo
	}
}

// WithAdditionalMetadata sets additional metadata to include
func WithAdditionalMetadata(metadata map[string]interface{}) Option {
	return func(cargo Cargo) Cargo {
		cargo.AdditionalMetadata = metadata
		return cargo
	}
}

// WithInstallArgs sets install args
func WithInstallArgs(args string) Option {
	return func(cargo Cargo) Cargo {
		cargo.InstallArgs = args
		return cargo
	}
}

// WithCargoService sets cargo service
func WithCargoService(s runner.CargoService) Option {
	return func(cargo Cargo) Cargo {
		cargo.CargoService = s
		return cargo
	}
}

// WithLogger sets logger
func WithLogger(l bard.Logger) Option {
	return func(cargo Cargo) Cargo {
		cargo.Logger = l
		return cargo
	}
}

// WithExcludeFolders sets logger
func WithExcludeFolders(f []string) Option {
	return func(cargo Cargo) Cargo {
		cargo.ExcludeFolders = f
		return cargo
	}
}

// WithStack sets logger
func WithStack(stack string) Option {
	return func(cargo Cargo) Cargo {
		cargo.Stack = stack
		return cargo
	}
}

type Cargo struct {
	AdditionalMetadata map[string]interface{}
	ApplicationPath    string
	Cache              Cache
	CargoService       runner.CargoService
	ExcludeFolders     []string
	InstallArgs        string
	LayerContributor   libpak.LayerContributor
	Logger             bard.Logger
	RunSBOMScan        bool
	SBOMScanner        sbom.SBOMScanner
	Stack              string
	WorkspaceMembers   string
}

// NewCargo creates a new cargo with the given options
func NewCargo(options ...Option) (Cargo, error) {
	cargo := Cargo{}

	for _, option := range options {
		cargo = option(cargo)
	}

	metadata := map[string]interface{}{
		"additional-arguments": cargo.InstallArgs,
		"workspace-members":    cargo.WorkspaceMembers,
		"stack":                cargo.Stack,
	}

	var err error
	metadata["files"], err = sherpa.NewFileListingHash(cargo.ApplicationPath)
	if err != nil {
		return Cargo{}, fmt.Errorf("unable to create file listing for %s\n%w", cargo.ApplicationPath, err)
	}

	metadata["cargo-version"], err = cargo.CargoService.CargoVersion()
	if err != nil {
		return Cargo{}, fmt.Errorf("unable to determine cargo version\n%w", err)
	}

	metadata["rust-version"], err = cargo.CargoService.RustVersion()
	if err != nil {
		return Cargo{}, fmt.Errorf("unable to determine rust version\n%w", err)
	}

	for k, v := range cargo.AdditionalMetadata {
		metadata[k] = v
	}

	cargo.LayerContributor = libpak.NewLayerContributor("Rust Application", metadata, libcnb.LayerTypes{
		Cache:  true,
		Launch: true,
	})
	cargo.LayerContributor.Logger = cargo.Logger

	return cargo, nil
}

func (c Cargo) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	layer, err := c.LayerContributor.Contribute(layer, func() (libcnb.Layer, error) {
		preserver := mtimes.NewPreserver(c.Logger)

		targetPath, err := os.Readlink(filepath.Join(c.ApplicationPath, "target"))
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to read target link\n%w", err)
		}

		cargoHome, found := os.LookupEnv("CARGO_HOME")
		if !found {
			return libcnb.Layer{}, fmt.Errorf("unable to find CARGO_HOME, it must be set")
		}

		err = preserver.RestoreAll(targetPath, cargoHome, layer.Path)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to restore all\n%w", err)
		}

		members, err := c.CargoService.WorkspaceMembers(c.ApplicationPath, layer)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to fetch members\n%w", err)
		}

		isPathSet, err := c.IsPathSet()
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to check if path set\n%w", err)
		}

		if len(members) == 0 {
			c.Logger.Body("WARNING: no members detected, trying to install with no path. This may fail.")
			// run `cargo install`
			err = c.CargoService.Install(c.ApplicationPath, layer)
			if err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to install default\n%w", err)
			}
		} else if (len(members) == 1 && members[0].Path == c.ApplicationPath) || isPathSet {
			// run `cargo install`
			err = c.CargoService.Install(c.ApplicationPath, layer)
			if err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to install single\n%w", err)
			}
		} else { // if len(members) > 1 and --path not set
			// run `cargo install --path=` for each member in the workspace
			for _, member := range members {
				err = c.CargoService.InstallMember(member.Path, c.ApplicationPath, layer)
				if err != nil {
					return libcnb.Layer{}, fmt.Errorf("unable to install member\n%w", err)
				}
			}
		}

		if c.RunSBOMScan {
			if err := c.SBOMScanner.ScanLayer(layer, c.ApplicationPath, libcnb.CycloneDXJSON, libcnb.SyftJSON); err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to create layer %s SBoM \n%w", layer.Name, err)
			}
		}

		err = preserver.PreserveAll(targetPath, cargoHome, layer.Path)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to preserve all\n%w", err)
		}

		return layer, nil
	})
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to contribute application layer\n%w", err)
	}

	c.Logger.Header("Removing source code")
	fs, err := ioutil.ReadDir(c.ApplicationPath)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to list children of %s\n%w", c.ApplicationPath, err)
	}

DELETE:
	for _, f := range fs {
		for _, excludeFolder := range c.ExcludeFolders {
			if f.Name() == excludeFolder {
				continue DELETE
			}
		}

		file := filepath.Join(c.ApplicationPath, f.Name())
		if err := os.RemoveAll(file); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to remove %s\n%w", file, err)
		}
	}

	if err := os.MkdirAll(filepath.Join(c.ApplicationPath, "bin"), 0755); err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable make app path %s/bin\n%w", c.ApplicationPath, err)
	}

	// symlink app files from layer to workspace
	err = filepath.Walk(filepath.Join(layer.Path, "bin"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		destPath := strings.Replace(path, layer.Path, c.ApplicationPath, 1)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		return os.Symlink(path, destPath)
	})
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to walk\n%w", err)
	}

	layer.LaunchEnvironment.Append("PATH", ":", filepath.Join(c.ApplicationPath, "bin"))

	return layer, nil
}

func (c Cargo) IsPathSet() (bool, error) {
	envArgs, err := runner.FilterInstallArgs(c.InstallArgs)
	if err != nil {
		return false, fmt.Errorf("unable to filter: %w", err)
	}

	for _, arg := range envArgs {
		if arg == "--path" || strings.HasPrefix(arg, "--path=") {
			return true, nil
		}
	}

	return false, nil
}

func (c Cargo) BuildProcessTypes(tiniEnabled bool) ([]libcnb.Process, error) {
	binaryTargets, err := c.CargoService.ProjectTargets(c.ApplicationPath)
	if err != nil {
		return []libcnb.Process{}, fmt.Errorf("unable to find project targets\n%w", err)
	}

	procs := []libcnb.Process{}
	for _, target := range binaryTargets {
		command := filepath.Join(c.ApplicationPath, "bin", target)
		args := []string{}
		if tiniEnabled {
			args = append([]string{"-g", "--", command}, args...)
			command = "tini"
		}
		procs = append(procs, libcnb.Process{
			Type:      target,
			Command:   command,
			Arguments: args,
			Direct:    true,
			Default:   false,
		})
	}

	if len(procs) > 0 {
		found := false
		for i := 0; i < len(procs) && !found; i++ {
			if procs[i].Type == "web" {
				procs[i].Default = true
				found = true
			}
		}

		if !found {
			procs[0].Default = true
		}
	}

	return procs, nil
}

func (c Cargo) Name() string {
	return "Cargo"
}
