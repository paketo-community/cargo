package main

import (
	"github.com/dmikusa/rust-cargo-cnb/cargo"
	"github.com/paketo-buildpacks/packit"
)

func main() {
	packit.Detect(cargo.Detect())
}
