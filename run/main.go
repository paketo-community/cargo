package main

import (
	"os"

	"github.com/dmikusa/rust-cargo-cnb/cargo"
	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/chronos"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/scribe"
)

func main() {
	cargoExe := pexec.NewExecutable("cargo")
	logger := scribe.NewEmitter(os.Stdout)

	packit.Run(
		cargo.Detect(),
		cargo.Build(cargo.NewCLIRunner(cargoExe), chronos.DefaultClock, logger),
	)
}
