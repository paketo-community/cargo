package cargo_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/dmikusa/rust-cargo-cnb/cargo"
	"github.com/paketo-buildpacks/packit"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir string
		detect     packit.DetectFunc
	)

	it.Before(func() {
		var err error
		workingDir, err = ioutil.TempDir("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		detect = cargo.Detect()
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	context("when there is a Cargo.toml and Cargo.lock file in the workspace", func() {
		it.Before(func() {
			_, err := os.Create(filepath.Join(workingDir, "Cargo.toml"))
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Create(filepath.Join(workingDir, "Cargo.lock"))
			Expect(err).NotTo(HaveOccurred())
		})

		it("returns a DetectResult that provides and required rust", func() {
			result, err := detect(packit.DetectContext{
				WorkingDir: workingDir,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.DetectResult{
				Plan: packit.BuildPlan{
					Provides: []packit.BuildPlanProvision{
						{Name: cargo.PlanDependencyRustCargo},
					},
					Requires: []packit.BuildPlanRequirement{
						{Name: cargo.PlanDependencyRustCargo},
						{Name: "rust"},
					},
				},
			}))
		})
	})

	context("failure cases", func() {
		context("Cargo.toml and Cargo.lock are missing", func() {
			it("returns an error", func() {
				_, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).To(MatchError(Equal("Missing [Cargo.toml: true, Cargo.lock: true], both required")))
			})
		})

		context("when there is a Cargo.toml without a Cargo.lock file", func() {
			it.Before(func() {
				err := ioutil.WriteFile(filepath.Join(workingDir, "Cargo.toml"), []byte{}, 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			it("returns an error", func() {
				_, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).To(MatchError("Missing [Cargo.toml: false, Cargo.lock: true], both required"))
			})
		})

		context("when there is a Cargo.lock without a Cargo.toml file", func() {
			it.Before(func() {
				err := ioutil.WriteFile(filepath.Join(workingDir, "Cargo.lock"), []byte{}, 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			it("returns an error", func() {
				_, err := detect(packit.DetectContext{WorkingDir: workingDir})
				Expect(err).To(MatchError("Missing [Cargo.toml: true, Cargo.lock: false], both required"))
			})
		})
	})
}
