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

package tini

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/sherpa"
)

type Tini struct {
	LayerContributor libpak.DependencyLayerContributor
	Logger           bard.Logger
}

func NewTini(dependency libpak.BuildpackDependency, cache libpak.DependencyCache) Tini {
	contributor := libpak.NewDependencyLayerContributor(dependency, cache, libcnb.LayerTypes{
		Launch: true,
	})
	return Tini{LayerContributor: contributor}
}

func (d Tini) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	d.LayerContributor.Logger = d.Logger

	return d.LayerContributor.Contribute(layer, func(artifact *os.File) (libcnb.Layer, error) {
		d.Logger.Bodyf("Copying to %s", layer.Path)

		err := os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to make bin directory\n%w", err)
		}

		file := filepath.Join(layer.Path, "bin", "tini")
		if err := sherpa.CopyFile(artifact, file); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to copy artifact to %s\n%w", file, err)
		}

		if err := os.Chmod(file, 0755); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to make tini executable\n%w", err)
		}

		return layer, nil
	})
}

func (d Tini) Name() string {
	return d.LayerContributor.LayerName()
}
