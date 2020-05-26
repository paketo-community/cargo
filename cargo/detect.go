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
			return packit.DetectResult{}, fmt.Errorf("Missing [Cargo.toml: %v, Cargo.lock: %v], both required", !cargoTomlFound, !cargoLockFound)
		}

		return packit.DetectResult{
			Plan: packit.BuildPlan{
				Provides: []packit.BuildPlanProvision{
					{Name: PlanDependencyRustCargo},
				},
				Requires: []packit.BuildPlanRequirement{
					{Name: PlanDependencyRustCargo},
					{Name: "rust"},
				},
			},
		}, nil
	}
}
