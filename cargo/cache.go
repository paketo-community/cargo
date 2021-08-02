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
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak/bard"
)

type Cache struct {
	Logger  bard.Logger
	AppPath string
}

func (c Cache) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	if err := os.MkdirAll(layer.Path, 0755); err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to create layer directory %s\n%w", layer.Path, err)
	}

	targetPath := filepath.Join(c.AppPath, "target")

	// delete the target if it exists as we'll never need it
	// users shouldn't push the target folder, but it can happen
	if err := os.RemoveAll(targetPath); err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to delete target directory\n%w", err)
	}

	// symlink the target folder to the cache layer, so we persist build info
	if err := os.Symlink(layer.Path, targetPath); err != nil {
		return libcnb.Layer{}, fmt.Errorf("unable to link cache from %s to %s\n%w", layer.Path, targetPath, err)
	} else {
		c.Logger.Bodyf("Creating cached target directory %s", targetPath)
	}

	layer.Cache = true
	return layer, nil
}

func (Cache) Name() string {
	return "Cargo Cache"
}
