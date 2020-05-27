package cargo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dmikusa/rust-cargo-cnb/cargo"
	"github.com/dmikusa/rust-cargo-cnb/cargo/mocks"
	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testCLIRunner(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		workingDir = "/does/not/matter"
		workLayer  = packit.Layer{Name: "work-layer", Path: "/some/location/1"}
		destLayer  = packit.Layer{Name: "dest-layer", Path: "/some/location/2"}
	)

	context("when there is a valid Rust project", func() {
		var runner cargo.CLIRunner

		it.Before(func() {
			env := os.Environ()
			env = append(env, `CARGO_TARGET_DIR=/some/location/1`)

			for i := 0; i < len(env); i++ {
				if strings.HasPrefix(env[i], "PATH=") {
					env[i] = fmt.Sprintf("%s%c%s", env[i], os.PathListSeparator, filepath.Join(destLayer.Path, "bin"))
				}
			}

			mockExe := mocks.Executable{}
			execution := pexec.Execution{
				Dir:    workingDir,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Args: []string{
					"install",
					"--color=never",
					"--path=.",
					"--root=/some/location/2",
				},
				Env: env,
			}
			mockExe.On("Execute", execution).Return(nil)
			runner = cargo.NewCLIRunner(&mockExe)
		})

		it("builds correctly", func() {
			err := runner.Install(workingDir, workLayer, destLayer)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	context("failure cases", func() {
		var runner cargo.CLIRunner

		it.Before(func() {
			env := os.Environ()
			env = append(env, `CARGO_TARGET_DIR=/some/location/1`)

			for i := 0; i < len(env); i++ {
				if strings.HasPrefix(env[i], "PATH=") {
					env[i] = fmt.Sprintf("%s%c%s", env[i], os.PathListSeparator, filepath.Join(destLayer.Path, "bin"))
				}
			}

			mockExe := mocks.Executable{}
			execution := pexec.Execution{
				Dir:    workingDir,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Args: []string{
					"install",
					"--color=never",
					"--path=.",
					"--root=/some/location/2",
				},
				Env: env,
			}
			mockExe.On("Execute", execution).Return(fmt.Errorf("expected"))
			runner = cargo.NewCLIRunner(&mockExe)
		})

		it("bubbles up failures", func() {
			err := runner.Install(workingDir, workLayer, destLayer)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(Equal("build failed: expected")))
		})
	})
}
