package cargo

import (
	"path/filepath"
	"time"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/scribe"
)

//go:generate mockery --name Summer --case=underscore

// Summer can make checksums of the item at the given path
type Summer interface {
	Sum(path ...string) (string, error)
}

//go:generate mockery --name Runner --case=underscore

// Runner is something capable of running Cargo
type Runner interface {
	Install(srcDir string, workLayer packit.Layer, destLayer packit.Layer) error
}

// Build does the actual install of Rust
func Build(runner Runner, summer Summer, clock chronos.Clock, logger scribe.Emitter) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)
		logger.Process("Cargo is checking if your Rust project needs to be built")

		cargoLayer, err := context.Layers.Get("rust-cargo")
		if err != nil {
			return packit.BuildResult{}, err
		}

		cargoLayer.Build = true
		cargoLayer.Cache = true

		binaryLayer, err := context.Layers.Get("rust-bin")
		if err != nil {
			return packit.BuildResult{}, err
		}

		binaryLayer.Launch = true

		cargoLockHash, err := summer.Sum(filepath.Join(context.WorkingDir, "Cargo.lock"))
		if err != nil {
			return packit.BuildResult{}, err
		}

		if sha, ok := cargoLayer.Metadata["cache_sha"].(string); !ok || sha != cargoLockHash {
			logger.Subprocess("Project needs to be built")
			logger.Break()

			then := clock.Now()
			err := runner.Install(context.WorkingDir, cargoLayer, binaryLayer)
			if err != nil {
				return packit.BuildResult{}, err
			}

			logger.Action("Completed in %s", time.Since(then).Round(time.Millisecond))
			logger.Break()

			cargoLayer.Metadata = map[string]interface{}{
				"built_at":  clock.Now().Format(time.RFC3339Nano),
				"cache_sha": cargoLockHash,
			}

			binaryLayer.Metadata = map[string]interface{}{
				"built_at": clock.Now().Format(time.RFC3339Nano),
			}
		} else {
			logger.Subprocess("No change, reusing")
			logger.Break()
		}

		return packit.BuildResult{
			Layers: []packit.Layer{
				cargoLayer,
				binaryLayer,
			},
		}, nil
	}
}
