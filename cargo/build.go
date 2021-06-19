package cargo

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dmikusa/rust-cargo-cnb/mtimes"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/scribe"
)

//go:generate mockery --name Runner --case=underscore

// Runner is something capable of running Cargo
type Runner interface {
	Install(srcDir string, workLayer packit.Layer, destLayer packit.Layer) error
	InstallMember(memberPath string, srcDir string, workLayer packit.Layer, destLayer packit.Layer) error
	WorkspaceMembers(srcDir string, workLayer packit.Layer, destLayer packit.Layer) ([]url.URL, error)
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
		err = preserver.Restore(cargoLayer.Path)
		if err != nil {
			return packit.BuildResult{}, err
		}

		members, err := runner.WorkspaceMembers(context.WorkingDir, cargoLayer, binaryLayer)
		if err != nil {
			return packit.BuildResult{}, err
		}

		isPathSet, err := IsPathSet()
		if err != nil {
			return packit.BuildResult{}, err
		}

		if len(members) == 0 {
			logger.Subprocess("WARNING: no members detected, trying to install with no path. This may fail.")
			// run `cargo install`
			err = runner.Install(context.WorkingDir, cargoLayer, binaryLayer)
			if err != nil {
				return packit.BuildResult{}, err
			}
		} else if (len(members) == 1 && members[0].Path == "/workspace") || isPathSet {
			// run `cargo install`
			err = runner.Install(context.WorkingDir, cargoLayer, binaryLayer)
			if err != nil {
				return packit.BuildResult{}, err
			}
		} else { // if len(members) > 1 and --path not set
			// run `cargo install --path=` for each member in the workspace
			for _, member := range members {
				err = runner.InstallMember(member.Path, context.WorkingDir, cargoLayer, binaryLayer)
				if err != nil {
					return packit.BuildResult{}, err
				}
			}
		}

		err = preserver.Preserve(cargoLayer.Path)
		if err != nil {
			return packit.BuildResult{}, err
		}

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

func IsPathSet() (bool, error) {
	envArgs, err := FilterInstallArgs(os.Getenv("BP_CARGO_INSTALL_ARGS"))
	if err != nil {
		return false, fmt.Errorf("filter failed: %w", err)
	}

	for _, arg := range envArgs {
		if arg == "--path" || strings.HasPrefix(arg, "--path=") {
			return true, nil
		}
	}

	return false, nil
}
