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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/paketo-community/cargo/cargo"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"
)

func testCargo(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx       libcnb.BuildContext
		executor  *mocks.Executor
		cargoHome string
	)

	it.Before(func() {
		var err error

		ctx.Layers.Path, err = ioutil.TempDir("", "cargo-layers")
		Expect(err).NotTo(HaveOccurred())

		ctx.Application.Path, err = ioutil.TempDir("", "app-dir")
		Expect(err).NotTo(HaveOccurred())

		cargoHome, err = ioutil.TempDir("", "cargo-home")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Setenv("CARGO_HOME", cargoHome)).ToNot(HaveOccurred())

		executor = &mocks.Executor{}
	})

	it.After(func() {
		Expect(os.Unsetenv("CARGO_HOME")).To(Succeed())
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
		Expect(os.RemoveAll(cargoHome)).To(Succeed())
	})

	context("contribution scenarios", func() {
		var (
			logger  bard.Logger
			cr      libpak.ConfigurationResolver
			appFile string
		)

		it.Before(func() {
			var err error

			logger = bard.NewLogger(ioutil.Discard)
			cr, err = libpak.NewConfigurationResolver(ctx.Buildpack, &logger)
			Expect(err).ToNot(HaveOccurred())

			executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
				return reflect.DeepEqual(ex.Args,
					[]string{"version"}) &&
					ex.Command == "cargo"
			})).Return(func(ex effect.Execution) error {
				_, err := ex.Stdout.Write([]byte("cargo 1.2.3 (4369396ce 2021-04-27)\n"))
				return err
			})

			executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
				return reflect.DeepEqual(ex.Args,
					[]string{"--version"}) &&
					ex.Command == "rustc"
			})).Return(func(ex effect.Execution) error {
				_, err := ex.Stdout.Write([]byte("rustc 1.2.3 (4369396ce 2021-04-27)\n"))
				return err
			})

			appFile = filepath.Join(ctx.Application.Path, "main.rs")
			Expect(ioutil.WriteFile(appFile, []byte{}, 0644)).To(Succeed())
		})

		context("validate metadata", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_CARGO_INSTALL_ARGS", "--path=./todo --foo=bar --foo baz")).To(Succeed())
				Expect(os.Setenv("BP_CARGO_WORKSPACE_MEMBERS", "foo, bar")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_CARGO_INSTALL_ARGS")).To(Succeed())
				Expect(os.Unsetenv("BP_CARGO_WORKSPACE_MEMBERS")).To(Succeed())
			})

			it("additional metadata and arguments are reflected through", func() {
				additionalMetadata := map[string]interface{}{
					"test": "expected-val",
				}

				r, err := cargo.NewCargo(
					additionalMetadata,
					ctx.Application.Path,
					nil,
					cargo.Cache{AppPath: ctx.Application.Path, Logger: logger},
					cargo.NewCLIRunner(cr, executor, logger),
					cr)
				Expect(err).ToNot(HaveOccurred())

				Expect(r.LayerContributor.ExpectedMetadata).To(HaveLen(6))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("cargo-version", "1.2.3"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("rust-version", "1.2.3"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("additional-arguments", "--path=./todo --foo=bar --foo baz"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("test", "expected-val"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("workspace-members", "foo, bar"))
				// can't reliably check hash value because it differs every time due to temp path changing on every test run
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKey("files"))
				Expect(r.LayerContributor.ExpectedMetadata.(map[string]interface{})["files"]).To(HaveLen(64))
			})
		})

		context("cargo workspace members", func() {
			var (
				r          cargo.Cargo
				layer      libcnb.Layer
				cacheLayer libcnb.Layer
			)

			it.Before(func() {
				var err error

				cache := cargo.Cache{AppPath: ctx.Application.Path, Logger: logger}
				cacheLayer, err = ctx.Layers.Layer("cache-layer")
				Expect(err).NotTo(HaveOccurred())
				cacheLayer, err = cache.Contribute(cacheLayer)
				Expect(err).NotTo(HaveOccurred())

				r, err = cargo.NewCargo(
					map[string]interface{}{},
					ctx.Application.Path,
					nil,
					cache,
					cargo.NewCLIRunner(cr, executor, logger),
					cr)
				Expect(err).ToNot(HaveOccurred())

				layer, err = ctx.Layers.Layer("cargo-layer")
				Expect(err).NotTo(HaveOccurred())
			})

			it("contributes cargo layer with no members", func() {
				var err error

				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, []string{"metadata", "--format-version=1", "--no-deps"})
				})).Return(func(ex effect.Execution) error {
					_, err := ex.Stdout.Write([]byte(BuildMetadata(ctx.Application.Path, []string{})))
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				expectedArgs := []string{"install", "--color=never", fmt.Sprintf("--root=%s", layer.Path), "--path=."}
				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, expectedArgs) &&
						ex.Command == "cargo"
				})).Return(func(ex effect.Execution) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				layer, err = r.Contribute(layer)
				Expect(err).NotTo(HaveOccurred())

				Expect(layer.LayerTypes.Cache).To(BeTrue())
				Expect(layer.LayerTypes.Build).To(BeFalse())
				Expect(layer.LayerTypes.Launch).To(BeFalse())

				// app files should be deleted
				Expect(appFile).ToNot(BeAnExistingFile())

				// preserver should have run
				Expect(filepath.Join(layer.Path, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

				// we should have two copies of the binary, one in the layer an one in the app root
				Expect(filepath.Join(layer.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

				executor := executor.Calls[3].Arguments[0].(effect.Execution)
				Expect(executor.Command).To(Equal("cargo"))
				Expect(executor.Args).To(Equal(expectedArgs))
			})

			it("contributes cargo layer with one member", func() {
				var err error

				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, []string{"metadata", "--format-version=1", "--no-deps"})
				})).Return(func(ex effect.Execution) error {
					_, err := ex.Stdout.Write([]byte(
						BuildMetadata(ctx.Application.Path,
							[]string{fmt.Sprintf("basics 2.0.0 (path+file://%s)", ctx.Application.Path)})))
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				expectedArgs := []string{"install", "--color=never", fmt.Sprintf("--root=%s", layer.Path), "--path=."}
				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, expectedArgs) &&
						ex.Command == "cargo"
				})).Return(func(ex effect.Execution) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				layer, err = r.Contribute(layer)
				Expect(err).NotTo(HaveOccurred())

				Expect(layer.LayerTypes.Cache).To(BeTrue())
				Expect(layer.LayerTypes.Build).To(BeFalse())
				Expect(layer.LayerTypes.Launch).To(BeFalse())

				// app files should be deleted
				Expect(appFile).ToNot(BeAnExistingFile())

				// preserver should have run
				Expect(filepath.Join(layer.Path, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

				// we should have two copies of the binary, one in the layer an one in the app root
				Expect(filepath.Join(layer.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

				executor := executor.Calls[3].Arguments[0].(effect.Execution)
				Expect(executor.Command).To(Equal("cargo"))
				Expect(executor.Args).To(Equal(expectedArgs))
			})

			context("--path is set", func() {
				it.Before(func() {
					Expect(os.Setenv("BP_CARGO_INSTALL_ARGS", "--path=./todo")).ToNot(HaveOccurred())
				})

				it.After(func() {
					Expect(os.Unsetenv("BP_CARGO_INSTALL_ARGS")).To(Succeed())
				})

				it("contributes cargo layer with multiples member but --path set", func() {
					var err error

					executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
						return reflect.DeepEqual(ex.Args, []string{"metadata", "--format-version=1", "--no-deps"})
					})).Return(func(ex effect.Execution) error {
						_, err := ex.Stdout.Write([]byte(
							BuildMetadata(ctx.Application.Path,
								[]string{
									fmt.Sprintf("basics 2.0.0 (path+file://%s/basics)", ctx.Application.Path),
									fmt.Sprintf("todo 1.2.0 (path+file://%s/todo)", ctx.Application.Path),
								})))
						Expect(err).ToNot(HaveOccurred())
						return nil
					})

					expectedArgs := []string{"install", "--path=./todo", "--color=never", fmt.Sprintf("--root=%s", layer.Path)}
					executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
						return reflect.DeepEqual(ex.Args, expectedArgs) &&
							ex.Command == "cargo"
					})).Return(func(ex effect.Execution) error {
						Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
						err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
						Expect(err).ToNot(HaveOccurred())
						return nil
					})

					layer, err = r.Contribute(layer)
					Expect(err).NotTo(HaveOccurred())

					Expect(layer.LayerTypes.Cache).To(BeTrue())
					Expect(layer.LayerTypes.Build).To(BeFalse())
					Expect(layer.LayerTypes.Launch).To(BeFalse())

					// app files should be deleted
					Expect(appFile).ToNot(BeAnExistingFile())

					// preserver should have run
					Expect(filepath.Join(layer.Path, "mtimes.json")).To(BeARegularFile())
					Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
					Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

					// we should have two copies of the binary, one in the layer an one in the app root
					Expect(filepath.Join(layer.Path, "bin", "my-binary")).To(BeARegularFile())
					Expect(filepath.Join(ctx.Application.Path, "bin", "my-binary")).To(BeARegularFile())
					Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

					executor := executor.Calls[3].Arguments[0].(effect.Execution)
					Expect(executor.Command).To(Equal("cargo"))
					Expect(executor.Args).To(Equal(expectedArgs))
				})
			})

			it("contributes cargo layer with multiple members", func() {
				var err error

				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, []string{"metadata", "--format-version=1", "--no-deps"})
				})).Return(func(ex effect.Execution) error {
					_, err := ex.Stdout.Write([]byte(
						BuildMetadata(ctx.Application.Path,
							[]string{
								fmt.Sprintf("basics 2.0.0 (path+file://%s/basics)", ctx.Application.Path),
								fmt.Sprintf("todo 2.0.0 (path+file://%s/todo)", ctx.Application.Path),
								fmt.Sprintf("hello 2.0.0 (path+file://%s/hello)", ctx.Application.Path)},
						)))
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				expectedArgsList := [3][]string{}

				expectedArgsList[0] = []string{
					"install",
					"--color=never",
					fmt.Sprintf("--root=%s", layer.Path),
					fmt.Sprintf("--path=%s", filepath.Join(ctx.Application.Path, "basics")),
				}
				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, expectedArgsList[0]) &&
						ex.Command == "cargo"
				})).Return(func(ex effect.Execution) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "basics"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				expectedArgsList[1] = []string{
					"install",
					"--color=never",
					fmt.Sprintf("--root=%s", layer.Path),
					fmt.Sprintf("--path=%s", filepath.Join(ctx.Application.Path, "todo")),
				}
				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, expectedArgsList[1]) &&
						ex.Command == "cargo"
				})).Return(func(ex effect.Execution) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "todo"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				expectedArgsList[2] = []string{
					"install",
					"--color=never",
					fmt.Sprintf("--root=%s", layer.Path),
					fmt.Sprintf("--path=%s", filepath.Join(ctx.Application.Path, "hello")),
				}
				executor.On("Execute", mock.MatchedBy(func(ex effect.Execution) bool {
					return reflect.DeepEqual(ex.Args, expectedArgsList[2]) &&
						ex.Command == "cargo"
				})).Return(func(ex effect.Execution) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "hello"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				layer, err = r.Contribute(layer)
				Expect(err).NotTo(HaveOccurred())

				Expect(layer.LayerTypes.Cache).To(BeTrue())
				Expect(layer.LayerTypes.Build).To(BeFalse())
				Expect(layer.LayerTypes.Launch).To(BeFalse())

				// app files should be deleted
				Expect(appFile).ToNot(BeAnExistingFile())

				// preserver should have run
				Expect(filepath.Join(layer.Path, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

				// we should have two copies of the binaries, one in the layer an one in the app root
				Expect(filepath.Join(layer.Path, "bin", "basics")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "basics")).To(BeARegularFile())
				Expect(filepath.Join(layer.Path, "bin", "todo")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "todo")).To(BeARegularFile())
				Expect(filepath.Join(layer.Path, "bin", "hello")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "hello")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

				for i := range expectedArgsList {
					// executor.Calls[3] is the first `cargo install` command
					executor := executor.Calls[3+i].Arguments[0].(effect.Execution)
					Expect(executor.Command).To(Equal("cargo"))

					Expect(executor.Args).To(Equal(expectedArgsList[i]))
				}
			})

			it("fails cause CARGO_HOME isn't set", func() {
				Expect(os.Unsetenv("CARGO_HOME")).To(Succeed())

				_, err := r.Contribute(layer)
				Expect(err).To(MatchError(ContainSubstring("unable to find CARGO_HOME, it must be set")))

				// app files should not be deleted
				Expect(appFile).To(BeAnExistingFile())

				// preserver should not have run
				Expect(filepath.Join(layer.Path, "mtimes.json")).ToNot(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).ToNot(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).ToNot(BeARegularFile())
			})
		})
	})
}
