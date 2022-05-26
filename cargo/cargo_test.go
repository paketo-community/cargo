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
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak/bard"
	sbomMocks "github.com/paketo-buildpacks/libpak/sbom/mocks"
	"github.com/paketo-community/cargo/cargo"
	"github.com/paketo-community/cargo/runner/mocks"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"
)

func testCargo(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx         libcnb.BuildContext
		service     *mocks.CargoService
		cargoHome   string
		logger      bard.Logger
		sbomScanner *sbomMocks.SBOMScanner
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

		logger = bard.NewLogger(io.Discard)

		service = &mocks.CargoService{}

		sbomScanner = &sbomMocks.SBOMScanner{}
	})

	it.After(func() {
		Expect(os.Unsetenv("CARGO_HOME")).To(Succeed())
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
		Expect(os.RemoveAll(cargoHome)).To(Succeed())
	})

	context("contribution scenarios", func() {
		var (
			appFile string
		)

		it.Before(func() {
			service.On("CargoVersion").Return("1.2.3", nil)
			service.On("RustVersion").Return("1.2.3", nil)

			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "src"), 0755)).To(Succeed())
			appFile = filepath.Join(ctx.Application.Path, "src", "main.rs")
			Expect(ioutil.WriteFile(appFile, []byte{}, 0644)).To(Succeed())
		})

		context("validate metadata", func() {
			it("additional metadata and arguments are reflected through", func() {
				additionalMetadata := map[string]interface{}{
					"test": "expected-val",
				}

				r, err := cargo.NewCargo(
					cargo.WithAdditionalMetadata(additionalMetadata),
					cargo.WithWorkspaceMembers("foo, bar"),
					cargo.WithApplicationPath(ctx.Application.Path),
					cargo.WithCargoService(service),
					cargo.WithInstallArgs("--path=./todo --foo=bar --foo baz"),
					cargo.WithStack("foo-stack"),
					cargo.WithTools([]string{"foo-tool"}),
					cargo.WithToolsArgs([]string{"--tool-arg"}),
					cargo.WithSBOMScanner(sbomScanner))

				Expect(err).ToNot(HaveOccurred())

				Expect(r.LayerContributor.ExpectedMetadata).To(HaveLen(9))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("cargo-version", "1.2.3"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("rust-version", "1.2.3"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("additional-arguments", "--path=./todo --foo=bar --foo baz"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("test", "expected-val"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("workspace-members", "foo, bar"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("stack", "foo-stack"))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("tools", []string{"foo-tool"}))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("tools-args", []string{"--tool-arg"}))
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKeyWithValue("stack", "foo-stack"))
				// can't reliably check hash value because it differs every time due to temp path changing on every test run
				Expect(r.LayerContributor.ExpectedMetadata).To(HaveKey("files"))
				Expect(r.LayerContributor.ExpectedMetadata.(map[string]interface{})["files"]).To(HaveLen(64))
			})
		})

		context("process types", func() {
			it("includes all binary targets as process types with first as default", func() {
				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"foo", "bar", "baz"}, nil)

				r, err := cargo.NewCargo(
					cargo.WithApplicationPath(ctx.Application.Path),
					cargo.WithCargoService(service),
					cargo.WithSBOMScanner(sbomScanner))
				Expect(err).ToNot(HaveOccurred())

				procs, err := r.BuildProcessTypes(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(procs).To(HaveLen(3))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "foo",
						Command:   filepath.Join(ctx.Application.Path, "bin", "foo"),
						Arguments: []string{},
						Direct:    true,
						Default:   true,
					}))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "bar",
						Command:   filepath.Join(ctx.Application.Path, "bin", "bar"),
						Arguments: []string{},
						Direct:    true,
						Default:   false,
					}))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "baz",
						Command:   filepath.Join(ctx.Application.Path, "bin", "baz"),
						Arguments: []string{},
						Direct:    true,
						Default:   false,
					}))
			})

			it("includes all binary targets as process types with web as default", func() {
				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"foo", "bar", "web", "baz"}, nil)

				r, err := cargo.NewCargo(
					cargo.WithApplicationPath(ctx.Application.Path),
					cargo.WithCargoService(service),
					cargo.WithSBOMScanner(sbomScanner))
				Expect(err).ToNot(HaveOccurred())

				procs, err := r.BuildProcessTypes(false)
				Expect(err).ToNot(HaveOccurred())

				Expect(procs).To(HaveLen(4))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "foo",
						Command:   filepath.Join(ctx.Application.Path, "bin", "foo"),
						Arguments: []string{},
						Direct:    true,
						Default:   false,
					}))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "bar",
						Command:   filepath.Join(ctx.Application.Path, "bin", "bar"),
						Arguments: []string{},
						Direct:    true,
						Default:   false,
					}))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "web",
						Command:   filepath.Join(ctx.Application.Path, "bin", "web"),
						Arguments: []string{},
						Direct:    true,
						Default:   true,
					}))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "baz",
						Command:   filepath.Join(ctx.Application.Path, "bin", "baz"),
						Arguments: []string{},
						Direct:    true,
						Default:   false,
					}))
			})

			it("includes all binary targets as process types run by tini with first as default", func() {
				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"foo", "bar", "baz"}, nil)

				r, err := cargo.NewCargo(
					cargo.WithApplicationPath(ctx.Application.Path),
					cargo.WithCargoService(service),
					cargo.WithSBOMScanner(sbomScanner))
				Expect(err).ToNot(HaveOccurred())

				procs, err := r.BuildProcessTypes(true)
				Expect(err).ToNot(HaveOccurred())

				Expect(procs).To(HaveLen(3))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "foo",
						Command:   "tini",
						Arguments: []string{"-g", "--", filepath.Join(ctx.Application.Path, "bin", "foo")},
						Direct:    true,
						Default:   true,
					}))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "bar",
						Command:   "tini",
						Arguments: []string{"-g", "--", filepath.Join(ctx.Application.Path, "bin", "bar")},
						Direct:    true,
						Default:   false,
					}))
				Expect(procs).To(ContainElement(
					libcnb.Process{
						Type:      "baz",
						Command:   "tini",
						Arguments: []string{"-g", "--", filepath.Join(ctx.Application.Path, "bin", "baz")},
						Direct:    true,
						Default:   false,
					}))
			})
		})

		context("cargo tools", func() {
			var (
				c          cargo.Cargo
				cacheLayer libcnb.Layer
			)

			it.Before(func() {
				var err error

				cache := cargo.Cache{AppPath: ctx.Application.Path, Logger: logger}
				cacheLayer, err = ctx.Layers.Layer("cache-layer")
				Expect(err).NotTo(HaveOccurred())
				cacheLayer, err = cache.Contribute(cacheLayer)
				Expect(err).NotTo(HaveOccurred())

				c, err = cargo.NewCargo(
					cargo.WithApplicationPath(ctx.Application.Path),
					cargo.WithCargoService(service),
					cargo.WithSBOMScanner(sbomScanner),
					cargo.WithTools([]string{"foo-tool"}),
					cargo.WithToolsArgs([]string{"--baz"}),
					cargo.WithRunSBOMScan(true))

				Expect(err).ToNot(HaveOccurred())
			})

			it("installs a tool", func() {
				service.On("InstallTool", "foo-tool", []string{"--baz"}).Return(nil)
				service.On("WorkspaceMembers", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return([]url.URL{}, nil)
				service.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return(func(srcDir string, layer libcnb.Layer) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})
				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"my-binary"}, nil)

				inputLayer, err := ctx.Layers.Layer("cargo-layer")
				Expect(err).ToNot(HaveOccurred())

				sbomScanner.On("ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON).Return(nil)

				_, err = c.Contribute(inputLayer)
				Expect(err).NotTo(HaveOccurred())

				Expect(service.Calls[2].Method).To(Equal("InstallTool"))
				Expect(service.Calls[2].Arguments[0]).To(Equal("foo-tool"))
				Expect(service.Calls[2].Arguments[1]).To(Equal([]string{"--baz"}))
			})
		})

		context("cargo workspace members", func() {
			var (
				c          cargo.Cargo
				cacheLayer libcnb.Layer
			)

			it.Before(func() {
				var err error

				cache := cargo.Cache{AppPath: ctx.Application.Path, Logger: logger}
				cacheLayer, err = ctx.Layers.Layer("cache-layer")
				Expect(err).NotTo(HaveOccurred())
				cacheLayer, err = cache.Contribute(cacheLayer)
				Expect(err).NotTo(HaveOccurred())

				c, err = cargo.NewCargo(
					cargo.WithApplicationPath(ctx.Application.Path),
					cargo.WithCargoService(service),
					cargo.WithSBOMScanner(sbomScanner),
					cargo.WithRunSBOMScan(true))

				Expect(err).ToNot(HaveOccurred())
			})

			it("contributes cargo layer with no members", func() {
				service.On("WorkspaceMembers", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return([]url.URL{}, nil)
				service.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return(func(srcDir string, layer libcnb.Layer) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})
				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"my-binary"}, nil)

				inputLayer, err := ctx.Layers.Layer("cargo-layer")
				Expect(err).ToNot(HaveOccurred())

				sbomScanner.On("ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON).Return(nil)

				outputLayer, err := c.Contribute(inputLayer)
				Expect(err).NotTo(HaveOccurred())

				sbomScanner.AssertCalled(t, "ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON)

				Expect(outputLayer.LayerTypes.Cache).To(BeTrue())
				Expect(outputLayer.LayerTypes.Build).To(BeFalse())
				Expect(outputLayer.LayerTypes.Launch).To(BeTrue())

				// app files should be deleted
				Expect(appFile).ToNot(BeAnExistingFile())

				// preserver should have run
				Expect(filepath.Join(outputLayer.Path, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

				// we should have two copies of the binary, one in the layer an one in the app root
				Expect(filepath.Join(outputLayer.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

				// Ensure `/workspace/bin` is added to the PATH at launch
				Expect(outputLayer.LaunchEnvironment["PATH.append"]).To(Equal(filepath.Join(ctx.Application.Path, "bin")))
			})

			it("contributes cargo layer with one member", func() {
				service.On("WorkspaceMembers", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return([]url.URL{
					{Scheme: "file", Path: filepath.Join(ctx.Application.Path)},
				}, nil)

				service.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return(func(srcDir string, layer libcnb.Layer) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"my-binary", "other"}, nil)

				inputLayer, err := ctx.Layers.Layer("cargo-layer")
				Expect(err).ToNot(HaveOccurred())

				sbomScanner.On("ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON).Return(nil)

				outputLayer, err := c.Contribute(inputLayer)
				Expect(err).NotTo(HaveOccurred())

				sbomScanner.AssertCalled(t, "ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON)

				Expect(outputLayer.LayerTypes.Cache).To(BeTrue())
				Expect(outputLayer.LayerTypes.Build).To(BeFalse())
				Expect(outputLayer.LayerTypes.Launch).To(BeTrue())

				// app files should be deleted
				Expect(appFile).ToNot(BeAnExistingFile())

				// preserver should have run
				Expect(filepath.Join(outputLayer.Path, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

				// we should have two copies of the binary, one in the layer an one in the app root
				Expect(filepath.Join(outputLayer.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

				// Ensure `/workspace/bin` is added to the PATH at launch
				Expect(outputLayer.LaunchEnvironment["PATH.append"]).To(Equal(filepath.Join(ctx.Application.Path, "bin")))
			})

			it("contributes cargo layer with one member without SBOM", func() {
				service.On("WorkspaceMembers", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return([]url.URL{
					{Scheme: "file", Path: filepath.Join(ctx.Application.Path)},
				}, nil)

				service.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return(func(srcDir string, layer libcnb.Layer) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"my-binary", "other"}, nil)

				inputLayer, err := ctx.Layers.Layer("cargo-layer")
				Expect(err).ToNot(HaveOccurred())

				c.RunSBOMScan = false

				outputLayer, err := c.Contribute(inputLayer)
				Expect(err).NotTo(HaveOccurred())

				sbomScanner.AssertNotCalled(t, "ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON)

				Expect(outputLayer.LayerTypes.Cache).To(BeTrue())
				Expect(outputLayer.LayerTypes.Build).To(BeFalse())
				Expect(outputLayer.LayerTypes.Launch).To(BeTrue())

				// app files should be deleted
				Expect(appFile).ToNot(BeAnExistingFile())

				// preserver should have run
				Expect(filepath.Join(outputLayer.Path, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

				// we should have two copies of the binary, one in the layer an one in the app root
				Expect(filepath.Join(outputLayer.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

				// Ensure `/workspace/bin` is added to the PATH at launch
				Expect(outputLayer.LaunchEnvironment["PATH.append"]).To(Equal(filepath.Join(ctx.Application.Path, "bin")))
			})

			context("--path is set", func() {
				it("contributes cargo layer with multiples member but --path set", func() {
					service.On("WorkspaceMembers", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return([]url.URL{
						{Scheme: "file", Path: filepath.Join(ctx.Application.Path, "basics")},
						{Scheme: "file", Path: filepath.Join(ctx.Application.Path, "todo")},
					}, nil)

					// include `--path`
					c.InstallArgs = "--path=./todo"

					service.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return(func(srcDir string, layer libcnb.Layer) error {
						Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
						err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
						Expect(err).ToNot(HaveOccurred())
						return nil
					})

					service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"my-binary"}, nil)

					inputLayer, err := ctx.Layers.Layer("cargo-layer")
					Expect(err).ToNot(HaveOccurred())

					sbomScanner.On("ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON).Return(nil)

					outputLayer, err := c.Contribute(inputLayer)
					Expect(err).NotTo(HaveOccurred())

					sbomScanner.AssertCalled(t, "ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON)

					Expect(outputLayer.LayerTypes.Cache).To(BeTrue())
					Expect(outputLayer.LayerTypes.Build).To(BeFalse())
					Expect(outputLayer.LayerTypes.Launch).To(BeTrue())

					// app files should be deleted
					Expect(appFile).ToNot(BeAnExistingFile())

					// preserver should have run
					Expect(filepath.Join(outputLayer.Path, "mtimes.json")).To(BeARegularFile())
					Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
					Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

					// we should have two copies of the binary, one in the layer an one in the app root
					Expect(filepath.Join(outputLayer.Path, "bin", "my-binary")).To(BeARegularFile())
					Expect(filepath.Join(ctx.Application.Path, "bin", "my-binary")).To(BeARegularFile())
					Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

					// Ensure `/workspace/bin` is added to the PATH at launch
					Expect(outputLayer.LaunchEnvironment["PATH.append"]).To(Equal(filepath.Join(ctx.Application.Path, "bin")))
				})
			})

			it("contributes cargo layer with multiple members", func() {
				service.On("WorkspaceMembers", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return([]url.URL{
					{Scheme: "file", Path: filepath.Join(ctx.Application.Path, "basics")},
					{Scheme: "file", Path: filepath.Join(ctx.Application.Path, "todo")},
					{Scheme: "file", Path: filepath.Join(ctx.Application.Path, "hello")},
				}, nil)

				service.On("InstallMember", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return(func(memberPath string, srcDir string, layer libcnb.Layer) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", filepath.Base(memberPath)), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"todo", "hello"}, nil)

				inputLayer, err := ctx.Layers.Layer("cargo-layer")
				Expect(err).ToNot(HaveOccurred())

				sbomScanner.On("ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON).Return(nil)

				outputLayer, err := c.Contribute(inputLayer)
				Expect(err).NotTo(HaveOccurred())

				sbomScanner.AssertCalled(t, "ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON)

				Expect(outputLayer.LayerTypes.Cache).To(BeTrue())
				Expect(outputLayer.LayerTypes.Build).To(BeFalse())
				Expect(outputLayer.LayerTypes.Launch).To(BeTrue())

				// app files should be deleted
				Expect(appFile).ToNot(BeAnExistingFile())

				// preserver should have run
				Expect(filepath.Join(outputLayer.Path, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

				// we should have two copies of the binaries, one in the layer an one in the app root
				Expect(filepath.Join(outputLayer.Path, "bin", "basics")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "basics")).To(BeARegularFile())
				Expect(filepath.Join(outputLayer.Path, "bin", "todo")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "todo")).To(BeARegularFile())
				Expect(filepath.Join(outputLayer.Path, "bin", "hello")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "hello")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())

				// make sure InstallMember ran three times
				service.AssertNumberOfCalls(t, "InstallMember", 3)

				// Ensure `/workspace/bin` is added to the PATH at launch
				Expect(outputLayer.LaunchEnvironment["PATH.append"]).To(Equal(filepath.Join(ctx.Application.Path, "bin")))
			})

			it("fails cause CARGO_HOME isn't set", func() {
				Expect(os.Unsetenv("CARGO_HOME")).To(Succeed())

				inputLayer, err := ctx.Layers.Layer("cargo-layer")
				Expect(err).ToNot(HaveOccurred())

				_, err = c.Contribute(inputLayer)
				Expect(err).To(MatchError(ContainSubstring("unable to find CARGO_HOME, it must be set")))

				// app files should not be deleted
				Expect(appFile).To(BeAnExistingFile())

				// preserver should not have run
				Expect(filepath.Join(inputLayer.Path, "mtimes.json")).ToNot(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).ToNot(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).ToNot(BeARegularFile())
			})
		})

		context("skip deleting certain app files", func() {
			var (
				c            cargo.Cargo
				cacheLayer   libcnb.Layer
				appFilesKeep []string
				appFilesGone []string
			)

			it.Before(func() {
				var err error

				cache := cargo.Cache{AppPath: ctx.Application.Path, Logger: logger}
				cacheLayer, err = ctx.Layers.Layer("cache-layer")
				Expect(err).NotTo(HaveOccurred())
				cacheLayer, err = cache.Contribute(cacheLayer)
				Expect(err).NotTo(HaveOccurred())

				appFilesKeep = []string{
					filepath.Join(ctx.Application.Path, "static", "index.html"),
					filepath.Join(ctx.Application.Path, "templates", "index.html"),
				}

				appFilesGone = []string{
					filepath.Join(ctx.Application.Path, "target", "stuff"),
					filepath.Join(ctx.Application.Path, "other", "file.txt"),
				}

				for _, appFile := range append(appFilesKeep, appFilesGone...) {
					Expect(os.MkdirAll(filepath.Dir(appFile), 0755)).To(Succeed())
					Expect(ioutil.WriteFile(appFile, []byte{}, 0644)).To(Succeed())
				}

				c, err = cargo.NewCargo(
					cargo.WithApplicationPath(ctx.Application.Path),
					cargo.WithCargoService(service),
					cargo.WithExcludeFolders([]string{"static", "templates"}),
					cargo.WithSBOMScanner(sbomScanner))

				Expect(err).ToNot(HaveOccurred())
			})

			it("doesn't delete skipped folders", func() {
				service.On("WorkspaceMembers", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return([]url.URL{
					{Scheme: "file", Path: filepath.Join(ctx.Application.Path)},
				}, nil)

				service.On("Install", mock.AnythingOfType("string"), mock.AnythingOfType("libcnb.Layer")).Return(func(srcDir string, layer libcnb.Layer) error {
					Expect(os.MkdirAll(filepath.Join(layer.Path, "bin"), 0755)).ToNot(HaveOccurred())
					err := ioutil.WriteFile(filepath.Join(layer.Path, "bin", "my-binary"), []byte("contents"), 0644)
					Expect(err).ToNot(HaveOccurred())
					return nil
				})

				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"my-binary", "other"}, nil)

				inputLayer, err := ctx.Layers.Layer("cargo-layer")
				Expect(err).ToNot(HaveOccurred())

				sbomScanner.On("ScanLayer", inputLayer, ctx.Application.Path, libcnb.CycloneDXJSON, libcnb.SyftJSON).Return(nil)

				outputLayer, err := c.Contribute(inputLayer)
				Expect(err).NotTo(HaveOccurred())

				Expect(outputLayer.LayerTypes.Cache).To(BeTrue())
				Expect(outputLayer.LayerTypes.Build).To(BeFalse())
				Expect(outputLayer.LayerTypes.Launch).To(BeTrue())

				// app files should be deleted
				Expect(appFile).ToNot(BeAnExistingFile())

				for _, appFile := range appFilesKeep {
					Expect(appFile).To(BeAnExistingFile())
				}

				for _, appFile := range appFilesGone {
					Expect(appFile).ToNot(BeAnExistingFile())
				}

				// preserver should have run
				Expect(filepath.Join(outputLayer.Path, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cargoHome, "mtimes.json")).To(BeARegularFile())
				Expect(filepath.Join(cacheLayer.Path, "mtimes.json")).To(BeARegularFile())

				// we should have two copies of the binary, one in the layer an one in the app root
				Expect(filepath.Join(outputLayer.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "bin", "my-binary")).To(BeARegularFile())
				Expect(filepath.Join(ctx.Application.Path, "mtimes.json")).ToNot(BeARegularFile())
			})
		})
	})
}
