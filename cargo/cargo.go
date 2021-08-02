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
	"github.com/paketo-buildpacks/libpak/sherpa"
	"github.com/paketo-community/cargo/mtimes"
)

type Cargo struct {
	ApplicationPath  string
	BOM              *libcnb.BOM
	Cache            Cache
	CLIRunner        CLIRunner
	ConfigResolver   libpak.ConfigurationResolver
	LayerContributor libpak.LayerContributor
	Logger           bard.Logger
}

func NewCargo(additionalMetadata map[string]interface{}, applicationPath string, bom *libcnb.BOM, cache Cache, cliRunner CLIRunner, configResolver libpak.ConfigurationResolver) (Cargo, error) {
	cargo := Cargo{
		ApplicationPath: applicationPath,
		BOM:             bom,
		Cache:           cache,
		CLIRunner:       cliRunner,
		ConfigResolver:  configResolver,
	}

	expected, err := cargo.expectedMetadata(additionalMetadata)
	if err != nil {
		return Cargo{}, fmt.Errorf("failed to generate expected metadata\n%w", err)
	}

	cargo.LayerContributor = libpak.NewLayerContributor("Rust Application", expected, libcnb.LayerTypes{
		Cache: true,
	})

	return cargo, nil
}

func (c Cargo) expectedMetadata(additionalMetadata map[string]interface{}) (map[string]interface{}, error) {
	var err error

	rawArgs, _ := c.ConfigResolver.Resolve("BP_CARGO_INSTALL_ARGS")
	rawMembers, _ := c.ConfigResolver.Resolve("BP_CARGO_WORKSPACE_MEMBERS")
	metadata := map[string]interface{}{
		"additional-arguments": rawArgs,
		"workspace-members":    rawMembers,
	}

	metadata["files"], err = sherpa.NewFileListingHash(c.ApplicationPath)
	if err != nil {
		return nil, fmt.Errorf("unable to create file listing for %s\n%w", c.ApplicationPath, err)
	}

	metadata["cargo-version"], err = c.CLIRunner.CargoVersion()
	if err != nil {
		return nil, fmt.Errorf("unable to determine cargo version\n%w", err)
	}

	metadata["rust-version"], err = c.CLIRunner.RustVersion()
	if err != nil {
		return nil, fmt.Errorf("unable to determine rust version\n%w", err)
	}

	for k, v := range additionalMetadata {
		metadata[k] = v
	}

	return metadata, nil
}

func (c Cargo) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	c.LayerContributor.Logger = c.Logger

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
			return libcnb.Layer{}, err
		}

		members, err := c.CLIRunner.WorkspaceMembers(c.ApplicationPath, layer)
		if err != nil {
			return libcnb.Layer{}, err
		}

		isPathSet, err := c.IsPathSet()
		if err != nil {
			return libcnb.Layer{}, err
		}

		if len(members) == 0 {
			c.Logger.Body("WARNING: no members detected, trying to install with no path. This may fail.")
			// run `cargo install`
			err = c.CLIRunner.Install(c.ApplicationPath, layer)
			if err != nil {
				return libcnb.Layer{}, err
			}
		} else if (len(members) == 1 && members[0].Path == c.ApplicationPath) || isPathSet {
			// run `cargo install`
			err = c.CLIRunner.Install(c.ApplicationPath, layer)
			if err != nil {
				return libcnb.Layer{}, err
			}
		} else { // if len(members) > 1 and --path not set
			// run `cargo install --path=` for each member in the workspace
			for _, member := range members {
				err = c.CLIRunner.InstallMember(member.Path, c.ApplicationPath, layer)
				if err != nil {
					return libcnb.Layer{}, err
				}
			}
		}

		err = preserver.PreserveAll(targetPath, cargoHome, layer.Path)
		if err != nil {
			return libcnb.Layer{}, err
		}

		return layer, nil
	})
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to contribute application layer\n%w", err)
	}

	// TODO: Add BOM stuff, cli_runner needs to do this & extract from Cargo.toml
	// entry, err := c.Cache.AsBOMEntry()
	// if err != nil {
	// 	return libcnb.Layer{}, fmt.Errorf("unable to generate build dependencies\n%w", err)
	// }
	// entry.Metadata["layer"] = c.Cache.Name()
	// c.BOM.Entries = append(c.BOM.Entries, entry)

	c.Logger.Header("Removing source code")
	fs, err := ioutil.ReadDir(c.ApplicationPath)
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to list children of %s\n%w", c.ApplicationPath, err)
	}
	for _, f := range fs {
		file := filepath.Join(c.ApplicationPath, f.Name())
		if err := os.RemoveAll(file); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to remove %s\n%w", file, err)
		}
	}

	// copy app files from layer to workspace
	err = filepath.Walk(filepath.Join(layer.Path, "bin"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil // skip directories sherpa will create directories
		}

		sourceFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("unable to open %s\n%w", path, err)
		}

		return sherpa.CopyFile(sourceFile,
			strings.Replace(path, layer.Path, c.ApplicationPath, 1))
	})
	if err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to walk\n%w", err)
	}

	return layer, nil
}

func (c Cargo) IsPathSet() (bool, error) {
	rawArgs, _ := c.ConfigResolver.Resolve("BP_CARGO_INSTALL_ARGS")

	envArgs, err := FilterInstallArgs(rawArgs)
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

func (c Cargo) Name() string {
	return "Cargo"
}
