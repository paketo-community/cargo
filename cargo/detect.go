package cargo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/packit"
)

// PlanDependencyRustCargo is the name of the plan
const PlanDependencyRustCargo = "rust-cargo"

// BuildPlanMetadata defines the information stored in the build plan
type BuildPlanMetadata struct {
	VersionSource string `toml:"version-source"`
	Version       string `toml:"version"`
}

// Detect if the Rust binaries should be delivered
func Detect() packit.DetectFunc {
	return func(context packit.DetectContext) (packit.DetectResult, error) {
		_, err := os.Stat(filepath.Join(context.WorkingDir, "Cargo.toml"))
		cargoTomlFound := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return packit.DetectResult{}, err
		}

		_, err = os.Stat(filepath.Join(context.WorkingDir, "Cargo.lock"))
		cargoLockFound := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return packit.DetectResult{}, err
		}

		if !cargoTomlFound || !cargoLockFound {
			//lint:ignore ST1005 Reads nicer when displayed to end user with leading capital letter
			return packit.DetectResult{}, fmt.Errorf("Missing [Cargo.toml: %v, Cargo.lock: %v], both required", !cargoTomlFound, !cargoLockFound)
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: PlanDependencyRustCargo},
				},
				Requires: []packit.BuildPlanRequirement{
					{Name: PlanDependencyRustCargo},
					{
						Name: "rust",
						Metadata: BuildPlanMetadata{
							Version:       "",
							VersionSource: "CARGO",
						},
					},
				},
			},
		}, nil
	}
}
