package mtimes_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitMTimes(t *testing.T) {
	suite := spec.New("MTimes", spec.Report(report.Terminal{}))
	suite("MTimes", testMTimes)
	suite.Run(t)
}
