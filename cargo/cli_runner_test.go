package cargo_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/dmikusa/rust-cargo-cnb/cargo"
	"github.com/dmikusa/rust-cargo-cnb/cargo/mocks"
	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-buildpacks/packit/pexec"
	"github.com/paketo-buildpacks/packit/scribe"
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

			args, err := runner.BuildArgs(destLayer, "foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(args).To(Equal([]string{
				"install",
				"--color=never",
				"--root=/some/location/2",
				"--path=foo",
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

				args, err := runner.BuildArgs(destLayer, ".")
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
			Expect(cargo.FilterInstallArgs("--root=somewhere")).To(BeEmpty())
			Expect(cargo.FilterInstallArgs("--root somewhere")).To(BeEmpty())
			Expect(cargo.FilterInstallArgs("--root=somewhere --root somewhere --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(cargo.FilterInstallArgs("--foo bar --root somewhere --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
		it("filters --color", func() {
			Expect(cargo.FilterInstallArgs("--color=never")).To(BeEmpty())
			Expect(cargo.FilterInstallArgs("--color always")).To(BeEmpty())
			Expect(cargo.FilterInstallArgs("--color=always --color never --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(cargo.FilterInstallArgs("--foo bar --color always --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
		it("filters both --color and --root", func() {
			Expect(cargo.FilterInstallArgs("--color=never --root=blah")).To(BeEmpty())
			Expect(cargo.FilterInstallArgs("--color always --root blah")).To(BeEmpty())
			Expect(cargo.FilterInstallArgs("--color=always --root=blah --root blah --color never --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(cargo.FilterInstallArgs("--foo bar --root=blah --color always --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
	})

	context("set default --path argument", func() {
		it("is specified by the user", func() {
			Expect(cargo.AddDefaultPath([]string{"install", "--path"}, ".")).To(Equal([]string{"install", "--path"}))
			Expect(cargo.AddDefaultPath([]string{"install", "--path=test"}, ".")).To(Equal([]string{"install", "--path=test"}))
			Expect(cargo.AddDefaultPath([]string{"install", "--path", "test"}, ".")).To(Equal([]string{"install", "--path", "test"}))
		})

		it("should be the default", func() {
			Expect(cargo.AddDefaultPath([]string{"install"}, ".")).To(Equal([]string{"install", "--path=."}))
			Expect(cargo.AddDefaultPath([]string{"install", "--foo=bar"}, ".")).To(Equal([]string{"install", "--foo=bar", "--path=."}))
			Expect(cargo.AddDefaultPath([]string{"install", "--foo", "bar"}, ".")).To(Equal([]string{"install", "--foo", "bar", "--path=."}))
		})
	})

	context("when there is a valid Rust project", func() {
		it("builds correctly with defaults", func() {
			logBuf := bytes.Buffer{}
			logger := scribe.NewEmitter(&logBuf)

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
			mockExe.On("Execute", mock.MatchedBy(func(ex pexec.Execution) bool {
				return reflect.DeepEqual(ex.Args, execution.Args) &&
					ex.Dir == execution.Dir &&
					reflect.DeepEqual(ex.Env, execution.Env) &&
					reflect.TypeOf(ex.Stdout) == reflect.TypeOf(scribe.Writer{})
			})).Return(nil)
			runner := cargo.NewCLIRunner(&mockExe, logger)

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
				logBuf := bytes.Buffer{}
				logger := scribe.NewEmitter(&logBuf)

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
				mockExe.On("Execute", mock.MatchedBy(func(ex pexec.Execution) bool {
					return reflect.DeepEqual(ex.Args, execution.Args) &&
						ex.Dir == execution.Dir &&
						reflect.DeepEqual(ex.Env, execution.Env) &&
						reflect.TypeOf(ex.Stdout) == reflect.TypeOf(scribe.Writer{})
				})).Return(nil)
				runner := cargo.NewCLIRunner(&mockExe, logger)

				err := runner.Install(workingDir, workLayer, destLayer)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		context("and there is metadata", func() {
			it("parses the member paths from metadata", func() {
				logBuf := bytes.Buffer{}
				logger := scribe.NewEmitter(&logBuf)

				metadata, err := ioutil.ReadFile("testdata/metadata.json")
				Expect(err).ToNot(HaveOccurred())

				mockExe := mocks.Executable{}
				mockExe.On("Execute", mock.MatchedBy(func(ex pexec.Execution) bool {
					Expect(ex.Args).To(Equal([]string{"metadata", "--format-version=1", "--no-deps"}))
					return true
				})).Return(func(ex pexec.Execution) error {
					_, err := ex.Stdout.Write(metadata)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				runner := cargo.NewCLIRunner(&mockExe, logger)
				urls, err := runner.WorkspaceMembers(workingDir, workLayer, destLayer)
				Expect(err).ToNot(HaveOccurred())

				Expect(urls).To(HaveLen(55))

				url, err := url.Parse("path+file:///Users/dmikusa/Code/Rust/actix-examples/basics/basics")
				Expect(err).ToNot(HaveOccurred())
				Expect(urls[0]).To(Equal(*url))

				url, err = url.Parse("path+file:///Users/dmikusa/Code/Rust/actix-examples/template_engines/tinytemplate")
				Expect(err).ToNot(HaveOccurred())
				Expect(urls[48]).To(Equal(*url))
			})
		})
	})

	context("failure cases", func() {
		it("bubbles up failures", func() {
			logBuf := bytes.Buffer{}
			logger := scribe.NewEmitter(&logBuf)

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
			mockExe.On("Execute", mock.MatchedBy(func(ex pexec.Execution) bool {
				return reflect.DeepEqual(ex.Args, execution.Args) &&
					ex.Dir == execution.Dir &&
					reflect.DeepEqual(ex.Env, execution.Env) &&
					reflect.TypeOf(ex.Stdout) == reflect.TypeOf(scribe.Writer{})
			})).Return(fmt.Errorf("expected"))
			runner := cargo.NewCLIRunner(&mockExe, logger)

			err := runner.Install(workingDir, workLayer, destLayer)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(Equal("build failed: expected")))
		})
	})

	context("when cargo home has files", func() {
		it("is cleaned up", func() {
			logBuf := bytes.Buffer{}
			logger := scribe.NewEmitter(&logBuf)

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

			err = cargo.NewCLIRunner(nil, logger).CleanCargoHomeCache(packit.Layer{Name: "Cargo", Path: workingDir})
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
			logBuf := bytes.Buffer{}
			logger := scribe.NewEmitter(&logBuf)

			workingDir, err := ioutil.TempDir("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			// To keep
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "bin"), 0755)).ToNot(HaveOccurred())

			// To destroy
			Expect(os.MkdirAll(filepath.Join(workingDir, "home", "baz"), 0755)).ToNot(HaveOccurred())

			err = cargo.NewCLIRunner(nil, logger).CleanCargoHomeCache(packit.Layer{Name: "Cargo", Path: workingDir})
			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Join(workingDir, "home", "bin")).To(BeADirectory())
			Expect(filepath.Join(workingDir, "home", "baz")).ToNot(BeADirectory())
		})
	})

	context("when specifying a subset of workspace members", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_CARGO_WORKSPACE_MEMBERS", "cookie-auth,protobuf-example, async_data_factory,hello-world")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_CARGO_WORKSPACE_MEMBERS")).To(Succeed())
		})

		it("filters workspace members", func() {
			logBuf := bytes.Buffer{}
			logger := scribe.NewEmitter(&logBuf)

			metadata, err := ioutil.ReadFile("testdata/metadata.json")
			Expect(err).ToNot(HaveOccurred())

			mockExe := mocks.Executable{}
			mockExe.On("Execute", mock.MatchedBy(func(ex pexec.Execution) bool {
				Expect(ex.Args).To(Equal([]string{"metadata", "--format-version=1", "--no-deps"}))
				return true
			})).Return(func(ex pexec.Execution) error {
				_, err := ex.Stdout.Write(metadata)
				Expect(err).ToNot(HaveOccurred())
				return nil
			})

			runner := cargo.NewCLIRunner(&mockExe, logger)
			urls, err := runner.WorkspaceMembers(workingDir, workLayer, destLayer)
			Expect(err).ToNot(HaveOccurred())

			Expect(urls).To(HaveLen(4))

			url, err := url.Parse("path+file:///Users/dmikusa/Code/Rust/actix-examples/basics/hello%20world")
			Expect(err).ToNot(HaveOccurred())
			Expect(urls[0]).To(Equal(*url))

			url, err = url.Parse("path+file:///Users/dmikusa/Code/Rust/actix-examples/other/data_factory")
			Expect(err).ToNot(HaveOccurred())
			Expect(urls[1]).To(Equal(*url))

			url, err = url.Parse("path+file:///Users/dmikusa/Code/Rust/actix-examples/other/protobuf")
			Expect(err).ToNot(HaveOccurred())
			Expect(urls[2]).To(Equal(*url))

			url, err = url.Parse("path+file:///Users/dmikusa/Code/Rust/actix-examples/session/cookie-auth")
			Expect(err).ToNot(HaveOccurred())
			Expect(urls[3]).To(Equal(*url))
		})
	})
}
