package mtimes

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/paketo-buildpacks/packit/scribe"
)

const PreserverMetadataFile = "mtimes.json"

// Preserver can be used to preserve the mtimes of a directory structure to a JSON file
type Preserver struct {
	Log scribe.Emitter
}

type Record struct {
	Path  string
	MTime time.Time
}

func NewPreserver(log scribe.Emitter) Preserver {
	return Preserver{
		Log: log,
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

		err = jsonEncoder.Encode(Record{path, fileInfo.ModTime()})
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

func (p Preserver) Restore(path string) error {
	metadataPath := filepath.Join(path, PreserverMetadataFile)
	fileIn, err := os.Open(metadataPath)
	if err != nil {
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
			p.Log.Detail("unable to restore time of file %s\n%w", r.Path, err)
		}
	}

	err = fileIn.Close()
	if err != nil {
		return fmt.Errorf("unable to close %s\n%w", metadataPath, err)
	}

	return nil
}
