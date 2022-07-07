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
	"github.com/heroku/color"
	"github.com/mattn/go-shellwords"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sbom"
	"github.com/paketo-community/cargo/runner"
	"github.com/paketo-community/cargo/tini"
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

		tiniEnabled := !cr.ResolveBool("BP_CARGO_TINI_DISABLED")
		if tiniEnabled {
			dr, err := libpak.NewDependencyResolver(context)
			if err != nil {
				return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency resolver\n%w", err)
			}

			dc, err := libpak.NewDependencyCache(context)
			if err != nil {
				return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency cache\n%w", err)
			}
			dc.Logger = b.Logger

			dep, err := dr.Resolve("tini", "")
			if err != nil {
				return libcnb.BuildResult{}, fmt.Errorf("unable to find dependency\n%w", err)
			}

			tini := tini.NewTini(dep, dc)
			tini.Logger = b.Logger
			result.Layers = append(result.Layers, tini)
		}

		cargoHome, found := cr.Resolve("CARGO_HOME")
		if !found {
			return libcnb.BuildResult{}, fmt.Errorf("unable to locate cargo home")
		}

		includeFolders, _ := cr.Resolve("BP_INCLUDE_FILES")

		// Deprecated: to be removed before the cargo 1.0.0 release
		deprecatedExcludeFolders, usedDeprecatedExclude := cr.Resolve("BP_CARGO_EXCLUDE_FOLDERS")
		if usedDeprecatedExclude {
			b.Logger.Infof("%s: `BP_CARGO_EXCLUDE_FOLDERS` has been deprecated and will be removed before the paketo-community/cargo 1.0 GA release. Use `BP_INCLUDE_FILES` instead.", color.YellowString("Warning"))
			includeFolders = fmt.Sprintf("%s:%s", includeFolders, strings.ReplaceAll(deprecatedExcludeFolders, ",", ":"))
		}

		excludeFolders, _ := cr.Resolve("BP_EXCLUDE_FILES")

		cargoWorkspaceMembers, _ := cr.Resolve("BP_CARGO_WORKSPACE_MEMBERS")
		cargoInstallArgs, _ := cr.Resolve("BP_CARGO_INSTALL_ARGS")
		skipSBOMScan := cr.ResolveBool("BP_DISABLE_SBOM")

		service := b.CargoService
		if service == nil {
			service = runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithCargoWorkspaceMembers(cargoWorkspaceMembers),
				runner.WithCargoInstallArgs(cargoInstallArgs),
				runner.WithExecutor(effect.NewExecutor()),
				runner.WithLogger(b.Logger),
				runner.WithStack(context.StackID))
		}

		cache := Cache{
			AppPath: context.Application.Path,
			Logger:  b.Logger,
		}
		result.Layers = append(result.Layers, cache)

		sbomScanner := sbom.NewSyftCLISBOMScanner(context.Layers, effect.NewExecutor(), b.Logger)

		cargoToolsRaw, _ := cr.Resolve("BP_CARGO_INSTALL_TOOLS")
		cargoTools, err := shellwords.Parse(cargoToolsRaw)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to parse BP_CARGO_INSTALL_TOOLS=%q\n%w", cargoToolsRaw, err)
		}

		cargoToolsArgsRaw, _ := cr.Resolve("BP_CARGO_INSTALL_TOOLS_ARGS")
		cargoToolsArgs, err := shellwords.Parse(cargoToolsArgsRaw)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to parse BP_CARGO_INSTALL_TOOLS_ARGS=%q\n%w", cargoToolsArgsRaw, err)
		}

		cargoLayer, err := NewCargo(
			WithApplicationPath(context.Application.Path),
			WithCargoService(service),
			WithIncludeFolders(includeFolders),
			WithExcludeFolders(excludeFolders),
			WithInstallArgs(cargoInstallArgs),
			WithLogger(b.Logger),
			WithRunSBOMScan(!skipSBOMScan),
			WithSBOMScanner(sbomScanner),
			WithStack(context.StackID),
			WithTools(cargoTools),
			WithToolsArgs(cargoToolsArgs),
			WithWorkspaceMembers(cargoWorkspaceMembers))
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to create cargo layer contributor\n%w", err)
		}

		result.Processes, err = cargoLayer.BuildProcessTypes(tiniEnabled)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to build list of process types\n%w", err)
		}

		result.Layers = append(result.Layers, cargoLayer)

		if skipSBOMScan {
			result.Labels = append(result.Labels, libcnb.Label{Key: "io.paketo.sbom.disabled", Value: "true"})
		}
	}

	return result, nil
}
