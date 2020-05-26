package cargo_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitRustCargo(t *testing.T) {
	suite := spec.New("Rust Cargo", spec.Report(report.Terminal{}))
	suite("Build", testBuild)
	suite("Detect", testDetect)
	suite("CLI Runner", testCLIRunner)
	suite.Run(t)
}
