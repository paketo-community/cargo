package cargo

import (
	"github.com/dmikusa/rust-cargo-cnb/mtimes"
	"time"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/scribe"
)

//go:generate mockery --name Runner --case=underscore

// Runner is something capable of running Cargo
type Runner interface {
	Install(srcDir string, workLayer packit.Layer, destLayer packit.Layer) error
}

// Build does the actual install of Rust
func Build(runner Runner, clock chronos.Clock, logger scribe.Emitter) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)
		logger.Process("Cargo is checking if your Rust project needs to be built")

		cargoLayer, err := context.Layers.Get("rust-cargo")
		if err != nil {
			return packit.BuildResult{}, err
		}

		cargoLayer.Cache = true

		binaryLayer, err := context.Layers.Get("rust-bin")
		if err != nil {
			return packit.BuildResult{}, err
		}

		binaryLayer.Launch = true

		then := clock.Now()
		preserver := mtimes.NewPreserver(logger)
		preserver.Restore(cargoLayer.Path)
		err = runner.Install(context.WorkingDir, cargoLayer, binaryLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}
		preserver.Preserve(cargoLayer.Path)

		logger.Action("Completed in %s", time.Since(then).Round(time.Millisecond))
		logger.Break()

		cargoLayer.Metadata = map[string]interface{}{
			"built_at": clock.Now().Format(time.RFC3339Nano),
		}

		binaryLayer.Metadata = map[string]interface{}{
			"built_at": clock.Now().Format(time.RFC3339Nano),
		}

		return packit.BuildResult{
			Layers: []packit.Layer{
				cargoLayer,
				binaryLayer,
			},
		}, nil
	}
}
