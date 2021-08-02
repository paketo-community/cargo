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

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
)

type Build struct {
	Logger bard.Logger
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

		cache := Cache{
			AppPath: context.Application.Path,
			Logger:  b.Logger,
		}
		result.Layers = append(result.Layers, cache)

		cargoLayer, err := NewCargo(
			map[string]interface{}{},
			context.Application.Path,
			nil,
			cache,
			NewCLIRunner(cr, effect.NewExecutor(), b.Logger),
			cr)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to create cargo layer contributor\n%w", err)
		}

		result.Layers = append(result.Layers, cargoLayer)
		// TODO: BOM support for Cargo
		// result.BOM.Entries = append(result.BOM.Entries, cargoBOM)
	}

	return result, nil
}
