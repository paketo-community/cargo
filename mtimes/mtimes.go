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

package mtimes

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/paketo-buildpacks/libpak/bard"
)

const PreserverMetadataFile = "mtimes.json"

// Preserver can be used to preserve the mtimes of a directory structure to a JSON file
type Preserver struct {
	Logger bard.Logger
}

type Record struct {
	Path  string
	MTime time.Time
}

func NewPreserver(logger bard.Logger) Preserver {
	return Preserver{
		Logger: logger,
	}
}

func (p Preserver) Preserve(path string) error {
	metadataPath := filepath.Join(path, PreserverMetadataFile)
	fileOut, err := os.Create(metadataPath)
	if err != nil {
		return fmt.Errorf("unable create metadata file %s\n%w", metadataPath, err)
	}
	defer fileOut.Close()

	jsonEncoder := json.NewEncoder(fileOut)

	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("unable read directory\n%w", err)
		}

		fileInfo, err := d.Info()
		if err != nil {
			return fmt.Errorf("unable to read file\n%w", err)
		}

		err = jsonEncoder.Encode(Record{path, fileInfo.ModTime().UTC()})
		if err != nil {
			return fmt.Errorf("unable to encode mtime\n%w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to recurse folder %s\n%w", path, err)
	}

	err = fileOut.Close()
	if err != nil {
		return fmt.Errorf("unable to close %s\n%w", metadataPath, err)
	}

	return nil
}

func (p Preserver) PreserveAll(paths ...string) error {
	for _, path := range paths {
		if err := p.Preserve(path); err != nil {
			return fmt.Errorf("unable to preserve path %s\n%w", path, err)
		}
	}

	return nil
}

func (p Preserver) Restore(path string) error {
	metadataPath := filepath.Join(path, PreserverMetadataFile)
	fileIn, err := os.Open(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			p.Logger.Body("File modification times not restored")
			return nil
		}
		return fmt.Errorf("unable open metadata file %s\n%w", metadataPath, err)
	}
	defer fileIn.Close()

	jsonDecoder := json.NewDecoder(fileIn)

	for jsonDecoder.More() {
		var r Record
		err := jsonDecoder.Decode(&r)
		if err != nil {
			return fmt.Errorf("unable to decode JSON\n%w", err)
		}

		err = os.Chtimes(r.Path, r.MTime, r.MTime)
		if err != nil {
			p.Logger.Bodyf("unable to restore time of file %s\n%w", r.Path, err)
		}
	}

	err = fileIn.Close()
	if err != nil {
		return fmt.Errorf("unable to close %s\n%w", metadataPath, err)
	}

	return nil
}

func (p Preserver) RestoreAll(paths ...string) error {
	for _, path := range paths {
		if err := p.Restore(path); err != nil {
			return fmt.Errorf("unable to restore path %s\n%w", path, err)
		}
	}

	return nil
}
