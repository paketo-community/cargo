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

package runner_test

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

	"github.com/buildpacks/libcnb"
	"github.com/paketo-community/cargo/runner"

	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/stretchr/testify/mock"

	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testRunners(t *testing.T, context spec.G, it spec.S) {
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

		runner := runner.NewCargoRunner(
			runner.WithCargoHome(cargoHome),
			runner.WithExecutor(executor),
			runner.WithLogger(bard.Logger{}))

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

		runner := runner.NewCargoRunner(
			runner.WithCargoHome(cargoHome),
			runner.WithExecutor(executor),
			runner.WithLogger(bard.Logger{}))

		version, err := runner.RustVersion()

		Expect(err).ToNot(HaveOccurred())
		Expect(version).To(Equal("1.2.3"))
	})

	context("builds install arguments", func() {
		it("builds a default set of arguments", func() {
			runner := runner.CargoRunner{}

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
			it("builds with custom args", func() {
				runner := runner.CargoRunner{
					CargoInstallArgs: "--path=./todo --foo=bar --foo baz",
				}

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
			Expect(runner.FilterInstallArgs("--root=somewhere")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--root somewhere")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--root=somewhere --root somewhere --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(runner.FilterInstallArgs("--foo bar --root somewhere --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
		it("filters --color", func() {
			Expect(runner.FilterInstallArgs("--color=never")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--color always")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--color=always --color never --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(runner.FilterInstallArgs("--foo bar --color always --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
		it("filters both --color and --root", func() {
			Expect(runner.FilterInstallArgs("--color=never --root=blah")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--color always --root blah")).To(BeEmpty())
			Expect(runner.FilterInstallArgs("--color=always --root=blah --root blah --color never --bar=baz")).To(Equal([]string{"--bar=baz"}))
			Expect(runner.FilterInstallArgs("--foo bar --root=blah --color always --baz --test true")).To(Equal([]string{"--foo", "bar", "--baz", "--test", "true"}))
		})
	})

	context("set default --path argument", func() {
		it("is specified by the user", func() {
			Expect(runner.AddDefaultPath([]string{"install", "--path"}, ".")).To(Equal([]string{"install", "--path"}))
			Expect(runner.AddDefaultPath([]string{"install", "--path=test"}, ".")).To(Equal([]string{"install", "--path=test"}))
			Expect(runner.AddDefaultPath([]string{"install", "--path", "test"}, ".")).To(Equal([]string{"install", "--path", "test"}))
		})

		it("should be the default", func() {
			Expect(runner.AddDefaultPath([]string{"install"}, ".")).To(Equal([]string{"install", "--path=."}))
			Expect(runner.AddDefaultPath([]string{"install", "--foo=bar"}, ".")).To(Equal([]string{"install", "--foo=bar", "--path=."}))
			Expect(runner.AddDefaultPath([]string{"install", "--foo", "bar"}, ".")).To(Equal([]string{"install", "--foo", "bar", "--path=."}))
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

			runner := runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithExecutor(executor),
				runner.WithLogger(logger))

			err := runner.Install(workingDir, destLayer)
			Expect(err).ToNot(HaveOccurred())
		})

		context("sets custom args", func() {
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

				runner := runner.NewCargoRunner(
					runner.WithCargoHome(cargoHome),
					runner.WithCargoInstallArgs("--path=./todo --foo=baz bar"),
					runner.WithExecutor(executor),
					runner.WithLogger(logger))

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

				runner := runner.NewCargoRunner(
					runner.WithCargoHome(cargoHome),
					runner.WithExecutor(executor),
					runner.WithLogger(logger))

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

					runner := runner.NewCargoRunner(
						runner.WithCargoHome(cargoHome),
						runner.WithCargoWorkspaceMembers("todo,jokes"),
						runner.WithExecutor(executor),
						runner.WithLogger(logger))

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

			runner := runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithExecutor(executor),
				runner.WithLogger(logger))

			err := runner.Install(workingDir, destLayer)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(Equal("unable to build\nexpected")))
		})
	})

	context("when cargo home has files", func() {
		it.After(func() {
			Expect(os.RemoveAll(cargoHome)).To(Succeed())
		})

		it("do nothing if CARGO_HOME is set but does not exist", func() {
			runner := runner.NewCargoRunner(
				runner.WithCargoHome("/foo/bar"),
				runner.WithExecutor(executor),
				runner.WithLogger(bard.Logger{}))

			Expect(runner.CleanCargoHomeCache()).To(BeNil())
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

			runner := runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithExecutor(executor),
				runner.WithLogger(bard.Logger{}))

			Expect(runner.CleanCargoHomeCache()).ToNot(HaveOccurred())
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

			runner := runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithExecutor(executor),
				runner.WithLogger(bard.Logger{}))

			Expect(runner.CleanCargoHomeCache()).ToNot(HaveOccurred())
			Expect(filepath.Join(cargoHome, "bin")).To(BeADirectory())
			Expect(filepath.Join(cargoHome, "baz")).ToNot(BeADirectory())
		})
	})

	context("package targets", func() {
		it("reads package target names", func() {
			metadata := BuildMetadataWithPackages("/does/not/matter",
				buildMetadata{
					members: []string{
						"basics 2.0.0 (path+file:///does/not/matter/basics)",
					},
					packages: []buildPackage{
						{
							id: "basics 2.0.0 (path+file:///does/not/matter/basics)",
							targets: []buildTarget{
								{kind: "lib", crateType: "lib", name: "inflector", srcPath: "/cargo_home/registry/src/github.com-1ecc6299db9ec823/Inflector-0.11.4/src/lib.rs", edition: "2015", doc: "true", doctest: "true", test: "true"},
								{kind: "bin", crateType: "bin", name: "decrypt", srcPath: "/does/not/matter/src/bin/decrypt/main.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
								{kind: "bin", crateType: "bin", name: "encrypt", srcPath: "/does/not/matter/src/bin/encrypt/main.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
								{kind: "bin", crateType: "bin", name: "pksign", srcPath: "/does/not/matter/src/bin/pksign/main.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
								{kind: "bin", crateType: "bin", name: "gcc-shim", srcPath: "/cargo_home/registry/src/github.com-1ecc6299db9ec823/cc-1.0.50/src/bin/gcc-shim.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
							},
						},
					},
				})

			executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
				Expect(ex.Args).To(Equal([]string{"metadata", "--format-version=1", "--no-deps"}))
				return true
			})).Return(func(ex effect.Execution) error {
				_, err := ex.Stdout.Write([]byte(metadata))
				Expect(err).ToNot(HaveOccurred())
				return nil
			})

			runner := runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithExecutor(executor),
				runner.WithLogger(bard.Logger{}))

			names, err := runner.ProjectTargets(workingDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(names).To(HaveLen(3))
			Expect(names).To(ContainElement("decrypt"))
			Expect(names).To(ContainElement("encrypt"))
			Expect(names).To(ContainElement("pksign"))
		})

		it("reads filtered target names", func() {
			metadata := BuildMetadataWithPackages("/does/not/matter",
				buildMetadata{
					members: []string{
						"basics 2.0.0 (path+file:///does/not/matter/basics)",
						"advanced 2.0.0 (path+file:///does/not/matter/advanced)",
						"expert 2.0.0 (path+file:///does/not/matter/expert)",
					},
					packages: []buildPackage{
						{
							id: "advanced 2.0.0 (path+file:///does/not/matter/advanced)",
							targets: []buildTarget{
								{kind: "lib", crateType: "lib", name: "inflector", srcPath: "/cargo_home/registry/src/github.com-1ecc6299db9ec823/Inflector-0.11.4/src/lib.rs", edition: "2015", doc: "true", doctest: "true", test: "true"},
								{kind: "bin", crateType: "bin", name: "decrypt", srcPath: "/does/not/matter/src/bin/decrypt/main.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
								{kind: "bin", crateType: "bin", name: "gcc-shim", srcPath: "/cargo_home/registry/src/github.com-1ecc6299db9ec823/cc-1.0.50/src/bin/gcc-shim.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
							},
						},
						{
							id: "expert 2.0.0 (path+file:///does/not/matter/expert)",
							targets: []buildTarget{
								{kind: "lib", crateType: "lib", name: "inflector", srcPath: "/cargo_home/registry/src/github.com-1ecc6299db9ec823/Inflector-0.11.4/src/lib.rs", edition: "2015", doc: "true", doctest: "true", test: "true"},
								{kind: "bin", crateType: "bin", name: "foo", srcPath: "/does/not/matter/src/bin/encrypt/main.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
								{kind: "bin", crateType: "bin", name: "bar", srcPath: "/does/not/matter/src/bin/pksign/main.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
								{kind: "bin", crateType: "bin", name: "gcc-shim", srcPath: "/cargo_home/registry/src/github.com-1ecc6299db9ec823/cc-1.0.50/src/bin/gcc-shim.rs", edition: "2018", doc: "true", doctest: "false", test: "true"},
							},
						},
					},
				})

			executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
				Expect(ex.Args).To(Equal([]string{"metadata", "--format-version=1", "--no-deps"}))
				return true
			})).Return(func(ex effect.Execution) error {
				_, err := ex.Stdout.Write([]byte(metadata))
				Expect(err).ToNot(HaveOccurred())
				return nil
			})

			runner := runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithExecutor(executor),
				runner.WithCargoWorkspaceMembers("expert"),
				runner.WithLogger(bard.Logger{}))

			names, err := runner.ProjectTargets(workingDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(names).To(HaveLen(2))
			Expect(names).To(ContainElement("foo"))
			Expect(names).To(ContainElement("bar"))
		})
	})

	context("workspace members", func() {
		it("default runs all workspaces", func() {
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

			runner := runner.NewCargoRunner(
				runner.WithCargoHome(cargoHome),
				runner.WithExecutor(executor),
				runner.WithLogger(bard.Logger{}))

			urls, err := runner.WorkspaceMembers(workingDir, destLayer)
			Expect(err).ToNot(HaveOccurred())
			Expect(urls).To(HaveLen(4))
		})

		context("when specifying a subset of workspace members", func() {
			it("filters workspace members", func() {
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

				runner := runner.NewCargoRunner(
					runner.WithCargoHome(cargoHome),
					runner.WithCargoWorkspaceMembers("basics,jokes"),
					runner.WithExecutor(executor),
					runner.WithLogger(bard.Logger{}))

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

type buildMetadata struct {
	members  []string
	packages []buildPackage
}

type buildPackage struct {
	id      string
	targets []buildTarget
}

type buildTarget struct {
	kind      string
	crateType string
	name      string
	srcPath   string
	edition   string
	doc       string
	doctest   string
	test      string
}

func BuildMetadata(workspacePath string, members []string) string {
	pkgs := []buildPackage{}
	for _, member := range members {
		pkgs = append(pkgs, buildPackage{id: member})
	}

	return BuildMetadataWithPackages(workspacePath, buildMetadata{
		members:  members,
		packages: pkgs,
	})
}

func BuildMetadataWithPackages(workspacePath string, data buildMetadata) string {
	tmp := `{
  "packages": %s,
  "workspace_root": "%s",
  "target_directory": "%s",
  "workspace_members": %s,
  "resolve": null,
  "version": 1,
  "metadata": null
}`

	memberJson := "["
	for i, member := range data.members {
		memberJson += fmt.Sprintf(`"%s"`, member)
		if i != len(data.members)-1 {
			memberJson += ","
		}
		memberJson += "\n"
	}
	memberJson += "]"

	packageJson := `[`
	for _, pkg := range data.packages {
		packageJson += fmt.Sprintf(`{"id": "%s", "targets": [ `, pkg.id)
		for i, t := range pkg.targets {
			packageJson += fmt.Sprintf(`{"kind": ["%s"], "crate_types": ["%s"], "name": "%s", "src_path": "%s", "edition": "%s", "doc": %s, "doctest": %s, "test": %s}`,
				t.kind, t.crateType, t.name, t.srcPath, t.edition, t.doc, t.doctest, t.test)
			if i != len(pkg.targets)-1 {
				packageJson += ","
			}
			packageJson += "\n"
		}
		packageJson += `]},`
	}
	packageJson = strings.Trim(packageJson, ",") + `]`

	return fmt.Sprintf(tmp, packageJson, workspacePath, filepath.Join(workspacePath, "target"), memberJson)
}
