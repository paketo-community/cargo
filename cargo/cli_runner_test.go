/*
 * Copyright 2018-2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cargo_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-community/cargo/cargo"

	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/stretchr/testify/mock"

	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testCLIRunner(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		workingDir = "/does/not/matter"
		destLayer  = libcnb.Layer{Name: "dest-layer", Path: "/some/location/2"}
		executor   *mocks.Executor
		cargoHome  string
	)

	it.Before(func() {
		var err error

		executor = &mocks.Executor{}

		cargoHome, err = ioutil.TempDir("", "working-dir")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Setenv("CARGO_HOME", cargoHome)).To(Succeed())
	})

	it.After(func() {
		Expect(os.Unsetenv("CARGO_HOME")).To(Succeed())
	})

	it("fetches cargo version", func() {
		execution := effect.Execution{
			Command: "cargo",
			Stdout:  &bytes.Buffer{},
			Args: []string{
				"version",
			},
		}
		executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
			return reflect.DeepEqual(ex.Args, execution.Args) && ex.Command == execution.Command
		})).Return(func(ex effect.Execution) error {
			_, err := ex.Stdout.Write([]byte("cargo 1.2.3 (4369396ce 2021-04-27)\n"))
			Expect(err).ToNot(HaveOccurred())
			return nil
		})

		runner := cargo.NewCLIRunner(
			libpak.ConfigurationResolver{},
			executor,
			bard.Logger{})
		version, err := runner.CargoVersion()

		Expect(err).ToNot(HaveOccurred())
		Expect(version).To(Equal("1.2.3"))
	})

	it("fetches Rust version", func() {
		execution := effect.Execution{
			Command: "rustc",
			Stdout:  &bytes.Buffer{},
			Args: []string{
				"--version",
			},
		}
		executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
			return reflect.DeepEqual(ex.Args, execution.Args) && ex.Command == execution.Command
		})).Return(func(ex effect.Execution) error {
			_, err := ex.Stdout.Write([]byte("rustc 1.2.3 (53cb7b09b 2021-06-17)\n"))
			Expect(err).ToNot(HaveOccurred())
			return nil
		})

		runner := cargo.NewCLIRunner(
			libpak.ConfigurationResolver{},
			executor,
			bard.Logger{})
		version, err := runner.RustVersion()

		Expect(err).ToNot(HaveOccurred())
		Expect(version).To(Equal("1.2.3"))
	})

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
			logger := bard.NewLogger(&logBuf)

			expectedArgs := []string{
				"install",
				"--color=never",
				"--root=/some/location/2",
				"--path=.",
			}
			executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
				return reflect.DeepEqual(ex.Args, expectedArgs) &&
					ex.Dir == workingDir
			})).Return(nil)

			runner := cargo.NewCLIRunner(
				libpak.ConfigurationResolver{
					Configurations: []libpak.BuildpackConfiguration{
						{Name: "BP_CARGO_INSTALL_ARGS", Build: true, Default: ""},
					},
				},
				executor,
				logger)

			err := runner.Install(workingDir, destLayer)
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
				logger := bard.NewLogger(&logBuf)

				expectedArgs := []string{
					"install",
					"--path=./todo",
					"--foo=baz",
					"bar",
					"--color=never",
					"--root=/some/location/2",
				}
				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, expectedArgs) &&
						ex.Dir == workingDir
				})).Return(nil)

				runner := cargo.NewCLIRunner(
					libpak.ConfigurationResolver{
						Configurations: []libpak.BuildpackConfiguration{
							{Name: "BP_CARGO_INSTALL_ARGS", Build: true, Default: ""},
						},
					},
					executor,
					logger)

				err := runner.Install(workingDir, destLayer)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		context("and there is metadata", func() {
			it("parses the member paths from metadata", func() {
				logBuf := bytes.Buffer{}
				logger := bard.NewLogger(&logBuf)

				metadata := BuildMetadata("/workspace",
					[]string{
						"basics 2.0.0 (path+file:///workspace/basics)",
						"todo 1.2.0 (path+file:///workspace/todo)",
						"routes 0.5.0 (path+file:///workspace/routes)",
						"jokes 1.5.6 (path+file:///workspace/jokes)",
					})

				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					Expect(ex.Args).To(Equal([]string{"metadata", "--format-version=1", "--no-deps"}))
					return true
				})).Return(func(ex effect.Execution) error {
					_, err := ex.Stdout.Write([]byte(metadata))
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				runner := cargo.NewCLIRunner(
					libpak.ConfigurationResolver{},
					executor,
					logger)
				urls, err := runner.WorkspaceMembers(workingDir, destLayer)
				Expect(err).ToNot(HaveOccurred())

				Expect(urls).To(HaveLen(4))

				url, err := url.Parse("path+file:///workspace/basics")
				Expect(err).ToNot(HaveOccurred())
				Expect(urls[0]).To(Equal(*url))

				url, err = url.Parse("path+file:///workspace/todo")
				Expect(err).ToNot(HaveOccurred())
				Expect(urls[1]).To(Equal(*url))

				url, err = url.Parse("path+file:///workspace/routes")
				Expect(err).ToNot(HaveOccurred())
				Expect(urls[2]).To(Equal(*url))

				url, err = url.Parse("path+file:///workspace/jokes")
				Expect(err).ToNot(HaveOccurred())
				Expect(urls[3]).To(Equal(*url))
			})

			context("member filter is set", func() {
				it.Before(func() {
					Expect(os.Setenv("BP_CARGO_WORKSPACE_MEMBERS", "todo,jokes")).ToNot(HaveOccurred())
				})

				it.After(func() {
					Expect(os.Unsetenv("BP_CARGO_WORKSPACE_MEMBERS")).To(Succeed())
				})

				it("parses the member paths from metadata and preserves order with filters", func() {
					logBuf := bytes.Buffer{}
					logger := bard.NewLogger(&logBuf)

					metadata := BuildMetadata("/workspace",
						[]string{
							"basics 2.0.0 (path+file:///workspace/basics)",
							"todo 1.2.0 (path+file:///workspace/todo)",
							"routes 0.5.0 (path+file:///workspace/routes)",
							"jokes 1.5.6 (path+file:///workspace/jokes)",
						})

					executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
						Expect(ex.Args).To(Equal([]string{"metadata", "--format-version=1", "--no-deps"}))
						return true
					})).Return(func(ex effect.Execution) error {
						_, err := ex.Stdout.Write([]byte(metadata))
						Expect(err).ToNot(HaveOccurred())
						return nil
					})

					runner := cargo.NewCLIRunner(
						libpak.ConfigurationResolver{},
						executor,
						logger)
					urls, err := runner.WorkspaceMembers(workingDir, destLayer)
					Expect(err).ToNot(HaveOccurred())

					Expect(urls).To(HaveLen(2))

					url, err := url.Parse("path+file:///workspace/todo")
					Expect(err).ToNot(HaveOccurred())
					Expect(urls[0]).To(Equal(*url))

					url, err = url.Parse("path+file:///workspace/jokes")
					Expect(err).ToNot(HaveOccurred())
					Expect(urls[1]).To(Equal(*url))
				})
			})
		})
	})

	context("failure cases", func() {
		it("bubbles up failures", func() {
			logBuf := bytes.Buffer{}
			logger := bard.NewLogger(&logBuf)

			expectedArgs := []string{
				"install",
				"--color=never",
				"--root=/some/location/2",
				"--path=.",
			}
			executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
				return reflect.DeepEqual(ex.Args, expectedArgs) &&
					ex.Dir == workingDir
			})).Return(fmt.Errorf("expected"))

			runner := cargo.NewCLIRunner(
				libpak.ConfigurationResolver{
					Configurations: []libpak.BuildpackConfiguration{
						{Name: "BP_CARGO_INSTALL_ARGS", Build: true, Default: ""},
					},
				},
				executor,
				logger)

			err := runner.Install(workingDir, destLayer)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(Equal("unable to build\nexpected")))
		})
	})

	context("when cargo home has files", func() {
		var (
			err    error
			logBuf bytes.Buffer
			logger bard.Logger
		)

		it.Before(func() {
			logBuf = bytes.Buffer{}
			logger = bard.NewLogger(&logBuf)
		})

		it.After(func() {
			Expect(os.RemoveAll(cargoHome)).To(Succeed())
		})

		it("fails if CARGO_HOME is not set", func() {
			os.Unsetenv("CARGO_HOME")

			err = cargo.NewCLIRunner(
				libpak.ConfigurationResolver{},
				nil,
				logger,
			).CleanCargoHomeCache()

			Expect(err).To(MatchError("unable to find CARGO_HOME"))
		})

		it("fails if CARGO_HOME is set but empty", func() {
			os.Setenv("CARGO_HOME", "  ")

			err = cargo.NewCLIRunner(
				libpak.ConfigurationResolver{},
				nil,
				logger,
			).CleanCargoHomeCache()

			Expect(err).To(MatchError("unable to find CARGO_HOME"))
		})

		it("do nothing if CARGO_HOME is set but does not exist", func() {
			os.Setenv("CARGO_HOME", "/foo/bar")

			err = cargo.NewCLIRunner(
				libpak.ConfigurationResolver{},
				nil,
				logger,
			).CleanCargoHomeCache()

			Expect(err).To(BeNil())
		})

		it("is cleaned up", func() {
			// To keep
			Expect(os.MkdirAll(filepath.Join(cargoHome, "bin"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(cargoHome, "registry", "index"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(cargoHome, "registry", "cache"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(cargoHome, "git", "db"), 0755)).ToNot(HaveOccurred())

			// To destroy
			Expect(os.MkdirAll(filepath.Join(cargoHome, "registry", "foo"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(cargoHome, "git", "bar"), 0755)).ToNot(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(cargoHome, "baz"), 0755)).ToNot(HaveOccurred())

			err = cargo.NewCLIRunner(
				libpak.ConfigurationResolver{},
				nil,
				logger,
			).CleanCargoHomeCache()

			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Join(cargoHome, "bin")).To(BeADirectory())
			Expect(filepath.Join(cargoHome, "registry", "index")).To(BeADirectory())
			Expect(filepath.Join(cargoHome, "registry", "cache")).To(BeADirectory())
			Expect(filepath.Join(cargoHome, "git", "db")).To(BeADirectory())
			Expect(filepath.Join(cargoHome, "registry", "foo")).ToNot(BeADirectory())
			Expect(filepath.Join(cargoHome, "git", "bar")).ToNot(BeADirectory())
			Expect(filepath.Join(cargoHome, "baz")).ToNot(BeADirectory())
		})

		it("handles when registry and git are not present", func() {
			// To keep
			Expect(os.MkdirAll(filepath.Join(cargoHome, "bin"), 0755)).ToNot(HaveOccurred())

			// To destroy
			Expect(os.MkdirAll(filepath.Join(cargoHome, "baz"), 0755)).ToNot(HaveOccurred())

			err = cargo.NewCLIRunner(
				libpak.ConfigurationResolver{
					Configurations: []libpak.BuildpackConfiguration{{Default: "*"}},
				},
				nil,
				logger,
			).CleanCargoHomeCache()

			Expect(err).ToNot(HaveOccurred())
			Expect(filepath.Join(cargoHome, "bin")).To(BeADirectory())
			Expect(filepath.Join(cargoHome, "baz")).ToNot(BeADirectory())
		})
	})

	context("workspace members", func() {
		it("default runs all workspaces", func() {
			logBuf := bytes.Buffer{}
			logger := bard.NewLogger(&logBuf)

			metadata := BuildMetadata("/workspace",
				[]string{
					"basics 2.0.0 (path+file:///workspace/basics)",
					"todo 1.2.0 (path+file:///workspace/todo)",
					"routes 0.5.0 (path+file:///workspace/routes)",
					"jokes 1.5.6 (path+file:///workspace/jokes)",
				})

			executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
				Expect(ex.Args).To(Equal([]string{"metadata", "--format-version=1", "--no-deps"}))
				return true
			})).Return(func(ex effect.Execution) error {
				_, err := ex.Stdout.Write([]byte(metadata))
				Expect(err).ToNot(HaveOccurred())
				return nil
			})

			runner := cargo.NewCLIRunner(
				libpak.ConfigurationResolver{
					Configurations: []libpak.BuildpackConfiguration{
						{Name: "BP_CARGO_WORKSPACE_MEMBERS", Build: true, Default: ""},
					},
				},
				executor,
				logger)

			urls, err := runner.WorkspaceMembers(workingDir, destLayer)
			Expect(err).ToNot(HaveOccurred())
			Expect(urls).To(HaveLen(4))
		})

		context("when specifying a subset of workspace members", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_CARGO_WORKSPACE_MEMBERS", "basics,jokes")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_CARGO_WORKSPACE_MEMBERS")).To(Succeed())
			})

			it("filters workspace members", func() {
				logBuf := bytes.Buffer{}
				logger := bard.NewLogger(&logBuf)

				metadata := BuildMetadata("/workspace",
					[]string{
						"basics 2.0.0 (path+file:///workspace/basics)",
						"todo 1.2.0 (path+file:///workspace/todo)",
						"routes 0.5.0 (path+file:///workspace/routes)",
						"jokes 1.5.6 (path+file:///workspace/jokes)",
					})

				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					Expect(ex.Args).To(Equal([]string{"metadata", "--format-version=1", "--no-deps"}))
					return true
				})).Return(func(ex effect.Execution) error {
					_, err := ex.Stdout.Write([]byte(metadata))
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				runner := cargo.NewCLIRunner(
					libpak.ConfigurationResolver{
						Configurations: []libpak.BuildpackConfiguration{
							{Name: "BP_CARGO_WORKSPACE_MEMBERS", Build: true, Default: ""},
						},
					},
					executor,
					logger)

				urls, err := runner.WorkspaceMembers(workingDir, destLayer)
				Expect(err).ToNot(HaveOccurred())

				Expect(urls).To(HaveLen(2))

				url, err := url.Parse("path+file:///workspace/basics")
				Expect(err).ToNot(HaveOccurred())
				Expect(urls[0]).To(Equal(*url))

				url, err = url.Parse("path+file:///workspace/jokes")
				Expect(err).ToNot(HaveOccurred())
				Expect(urls[1]).To(Equal(*url))
			})
		})
	})
}

func BuildMetadata(workspacePath string, members []string) string {
	tmp := `{
  "packages": [],
  "workspace_root": "%s",
  "target_directory": "%s",
  "workspace_members": %s,
  "resolve": null,
  "version": 1,
  "metadata": null
}`

	memberJson := "["
	for i, member := range members {
		memberJson += fmt.Sprintf(`"%s"`, member)
		if i != len(members)-1 {
			memberJson += ","
		}
		memberJson += "\n"
	}
	memberJson += "]"

	return fmt.Sprintf(tmp, workspacePath, filepath.Join(workspacePath, "target"), memberJson)
}
