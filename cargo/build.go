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
	"strings"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-community/cargo/runner"
)

type Build struct {
	CargoService runner.CargoService
	Logger       bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	b.Logger.Title(context.Buildpack)
	result := libcnb.NewBuildResult()

	pr := libpak.PlanEntryResolver{Plan: context.Plan}

	if _, ok, err := pr.Resolve(PlanEntryRustCargo); err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to resolve Rust Cargo plan entry\n%w", err)
	} else if ok {
		cr, err := libpak.NewConfigurationResolver(context.Buildpack, &b.Logger)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
		}

		cargoHome, found := cr.Resolve("CARGO_HOME")
		if !found {
			return libcnb.BuildResult{}, fmt.Errorf("unable to locate cargo home")
		}

		excludeFoldersRaw, _ := cr.Resolve("BP_CARGO_EXCLUDE_FOLDERS")
		var excludeFolders []string
		for _, excludeFolder := range strings.Split(excludeFoldersRaw, ",") {
			excludeFolders = append(excludeFolders, strings.TrimSpace(excludeFolder))
		}

		cargoWorkspaceMembers, _ := cr.Resolve("BP_CARGO_WORKSPACE_MEMBERS")
		cargoInstallArgs, _ := cr.Resolve("BP_CARGO_INSTALL_ARGS")

		service := b.CargoService
		if service == nil {
			service = runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithCargoWorkspaceMembers(cargoWorkspaceMembers),
				runner.WithCargoInstallArgs(cargoInstallArgs),
				runner.WithExecutor(effect.NewExecutor()),
				runner.WithLogger(b.Logger))
		}

		cache := Cache{
			AppPath: context.Application.Path,
			Logger:  b.Logger,
		}
		result.Layers = append(result.Layers, cache)

		cargoLayer, err := NewCargo(
			WithInstallArgs(cargoInstallArgs),
			WithWorkspaceMembers(cargoWorkspaceMembers),
			WithApplicationPath(context.Application.Path),
			WithLogger(b.Logger),
			WithCargoService(service),
			WithExcludeFolders(excludeFolders))
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to create cargo layer contributor\n%w", err)
		}

		result.Processes, err = cargoLayer.BuildProcessTypes()
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to build list of process types\n%w", err)
		}

		result.Layers = append(result.Layers, cargoLayer)
		// TODO: BOM support for Cargo
		// result.BOM.Entries = append(result.BOM.Entries, cargoBOM)
	}

	return result, nil
}
