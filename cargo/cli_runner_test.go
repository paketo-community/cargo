package cargo_test

import (
	"fmt"
	"io/ioutil"
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

	context("builds install arguments", func() {
		it("builds a default set of arguments", func() {
			runner := cargo.CLIRunner{}

			args, err := runner.BuildArgs(destLayer)
			Expect(err).ToNot(HaveOccurred())
			Expect(args).To(Equal([]string{
				"install",
				"--color=never",
				"--root=/some/location/2",
				"--path=.",
			}))
		})

		context("with custom args", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_CARGO_INSTALL_ARGS", "--path=./todo --foo=bar --foo baz")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_CARGO_INSTALL_ARGS")).To(Succeed())
			})

			it("builds with custom args", func() {
				runner := cargo.CLIRunner{}

				args, err := runner.BuildArgs(destLayer)
				Expect(err).ToNot(HaveOccurred())
				Expect(args).To(Equal([]string{
					"install",
					"--path=./todo",
					"--foo=bar",
					"--foo",
					"baz",
					"--color=never",
					"--root=/some/location/2",
				}))
			})
		})
	})

	context("BP_CARGO_INSTALL_ARGS filters --color and --root", func() {
		it("filters --root", func() {
			runner := cargo.CLIRunner{}

			Expect(runner.FilterInstallArgs("--root=somewhere")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--root somewhere")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--root=somewhere --root somewhere --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(runner.FilterInstallArgs("--foo bar --root somewhere --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
		it("filters --color", func() {
			runner := cargo.CLIRunner{}

			Expect(runner.FilterInstallArgs("--color=never")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--color always")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--color=always --color never --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(runner.FilterInstallArgs("--foo bar --color always --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
		it("filters both --color and --root", func() {
			runner := cargo.CLIRunner{}

			Expect(runner.FilterInstallArgs("--color=never --root=blah")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--color always --root blah")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--color=always --root=blah --root blah --color never --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(runner.FilterInstallArgs("--foo bar --root=blah --color always --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
	})

	context("set default --path argument", func() {
		it("is specified by the user", func() {
			runner := cargo.CLIRunner{}

			Expect(runner.AddDefaultPath([]string{"install", "--path"})).To(Equal([]string{"install", "--path"}))
			Expect(runner.AddDefaultPath([]string{"install", "--path=test"})).To(Equal([]string{"install", "--path=test"}))
			Expect(runner.AddDefaultPath([]string{"install", "--path", "test"})).To(Equal([]string{"install", "--path", "test"}))
		})

		it("should be the default", func() {
			runner := cargo.CLIRunner{}

			Expect(runner.AddDefaultPath([]string{"install"})).To(Equal([]string{"install", "--path=."}))
			Expect(runner.AddDefaultPath([]string{"install", "--foo=bar"})).To(Equal([]string{"install", "--foo=bar", "--path=."}))
			Expect(runner.AddDefaultPath([]string{"install", "--foo", "bar"})).To(Equal([]string{"install", "--foo", "bar", "--path=."}))
		})
	})

	context("when there is a valid Rust project", func() {
		it("builds correctly with defaults", func() {
			env := os.Environ()
			env = append(env, `CARGO_TARGET_DIR=/some/location/1/target`)
			env = append(env, `CARGO_HOME=/some/location/1/home`)

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
					"--root=/some/location/2",
					"--path=.",
				},
				Env: env,
			}
			mockExe.On("Execute", execution).Return(nil)
			runner := cargo.NewCLIRunner(&mockExe)

			err := runner.Install(workingDir, workLayer, destLayer)
			Expect(err).ToNot(HaveOccurred())
		})

		context("sets custom args", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_CARGO_INSTALL_ARGS", "--path=./todo --foo=baz bar")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_CARGO_INSTALL_ARGS")).To(Succeed())
			})

			it("builds correctly with custom args", func() {
				env := os.Environ()
				env = append(env, `CARGO_TARGET_DIR=/some/location/1/target`)
				env = append(env, `CARGO_HOME=/some/location/1/home`)

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
						"--path=./todo",
						"--foo=baz",
						"bar",
						"--color=never",
						"--root=/some/location/2",
					},
					Env: env,
				}
				mockExe.On("Execute", execution).Return(nil)
				runner := cargo.NewCLIRunner(&mockExe)

				err := runner.Install(workingDir, workLayer, destLayer)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	context("failure cases", func() {
		it("bubbles up failures", func() {
			env := os.Environ()
			env = append(env, `CARGO_TARGET_DIR=/some/location/1/target`)
			env = append(env, `CARGO_HOME=/some/location/1/home`)

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
					"--root=/some/location/2",
					"--path=.",
				},
				Env: env,
			}
			mockExe.On("Execute", execution).Return(fmt.Errorf("expected"))
			runner := cargo.NewCLIRunner(&mockExe)

			err := runner.Install(workingDir, workLayer, destLayer)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(Equal("build failed: expected")))
		})
	})

	context("when cargo home has files", func() {
		it("is cleaned up", func() {
			workingDir, err := ioutil.TempDir("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			// To keep
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "bin"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "registry", "index"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "registry", "cache"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "git", "db"), 0755)).ToNot(HaveOccurred())

			// To destroy
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "registry", "foo"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "git", "bar"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "baz"), 0755)).ToNot(HaveOccurred())

			err = cargo.NewCLIRunner(nil).CleanCargoHomeCache(packit.Layer{Name: "Cargo", Path: workingDir})
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Join(workingDir, "home", "bin")).To(BeADirectory())
			Expect(filepath.Join(workingDir, "home", "registry", "index")).To(BeADirectory())
			Expect(filepath.Join(workingDir, "home", "registry", "cache")).To(BeADirectory())
			Expect(filepath.Join(workingDir, "home", "git", "db")).To(BeADirectory())
			Expect(filepath.Join(workingDir, "home", "registry", "foo")).ToNot(BeADirectory())
			Expect(filepath.Join(workingDir, "home", "git", "bar")).ToNot(BeADirectory())
			Expect(filepath.Join(workingDir, "home", "baz")).ToNot(BeADirectory())
		})

		it("handles when registry and git are not present", func() {
			workingDir, err := ioutil.TempDir("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			// To keep
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "bin"), 0755)).ToNot(HaveOccurred())

			// To destroy
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "baz"), 0755)).ToNot(HaveOccurred())

			err = cargo.NewCLIRunner(nil).CleanCargoHomeCache(packit.Layer{Name: "Cargo", Path: workingDir})
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Join(workingDir, "home", "bin")).To(BeADirectory())
			Expect(filepath.Join(workingDir, "home", "baz")).ToNot(BeADirectory())
		})
	})
}
