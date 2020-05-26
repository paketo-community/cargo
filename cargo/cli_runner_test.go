package cargo_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/dmikusa/rust-cargo-cnb/cargo"
	"github.com/dmikusa/rust-cargo-cnb/cargo/mocks"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testCLIRunner(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir = "/does/not/matter"
	)

	context("when there is a valid Rust project", func() {
		var runner cargo.CLIRunner

		it.Before(func() {
			mockExe := mocks.Executable{}
			execution := pexec.Execution{
				Dir:    workingDir,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Args:   []string{},
			}
			mockExe.On("Execute", execution).Return(nil)
			runner = cargo.NewCLIRunner(&mockExe)
		})

		it("builds correctly", func() {
			err := runner.Build(workingDir)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	context("failure cases", func() {
		var runner cargo.CLIRunner

		it.Before(func() {
			mockExe := mocks.Executable{}
			execution := pexec.Execution{
				Dir:    workingDir,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Args:   []string{},
			}
			mockExe.On("Execute", execution).Return(fmt.Errorf("expected"))
			runner = cargo.NewCLIRunner(&mockExe)
		})

		it("bubbles up failures", func() {
			err := runner.Build(workingDir)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(Equal("build failed: expected")))
		})
	})
}
