package main

import (
	"os"

	rustCargo "github.com/dmikusa/rust-cargo-cnb/cargo"
	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/fs"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/scribe"
)

func main() {
	cargoExe := pexec.NewExecutable("cargo")
	checksumCalculator := fs.NewChecksumCalculator()
	logger := scribe.NewEmitter(os.Stdout)

	packit.Build(rustCargo.Build(rustCargo.NewCLIRunner(cargoExe), checksumCalculator, chronos.DefaultClock, logger))
}
