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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-community/cargo/cargo"
	"github.com/paketo-community/cargo/runner/mocks"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx        libcnb.BuildContext
		cargoBuild cargo.Build
		service    mocks.CargoService
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "build-application")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "bin"), 0755)).ToNot(HaveOccurred())

		ctx.Layers.Path, err = ioutil.TempDir("", "build-layers")
		Expect(err).NotTo(HaveOccurred())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"dependencies": []map[string]interface{}{
				{
					"id":      "tini",
					"version": "1.1.1",
					"stacks":  []interface{}{"test-stack-id"},
					"cpes":    []string{"cpe:2.3:a:tini:tini:1.1.1:*:*:*:*:*:*:*"},
					"purl":    "pkg:generic/tini@1.1.1",
				},
			},
			"configurations": []map[string]interface{}{
				{"name": "BP_CARGO_TINI_DISABLED", "default": "false"},
			},
		}
		ctx.StackID = "test-stack-id"

		service = mocks.CargoService{}

		cargoBuild = cargo.Build{
			Logger:       bard.NewLogger(&bytes.Buffer{}),
			CargoService: &service,
		}

		service.On("CargoVersion").Return("1.2.3", nil)
		service.On("RustVersion").Return("1.2.3", nil)
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("does not contribute anything", func() {
		result, err := cargoBuild.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(HaveLen(0))
	})

	context("build plan entry exists", func() {
		it.Before(func() {
			Expect(os.Setenv("CARGO_HOME", "/does/not/matter")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("CARGO_HOME")).To(Succeed())
		})

		it("contributes cargo layer", func() {
			ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "rust-cargo"})

			service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"app1", "app2", "app3"}, nil)

			result, err := cargoBuild.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(3))
			Expect(result.Layers[0].Name()).To(Equal("tini"))
			Expect(result.Layers[1].Name()).To(Equal("Cargo Cache"))
			Expect(result.Layers[2].Name()).To(Equal("Cargo"))

			Expect(result.Processes).To(HaveLen(3))
			Expect(result.Processes).To(ContainElement(
				libcnb.Process{
					Type:    "app1",
					Command: "tini",
					Arguments: []string{
						"-g",
						"--",
						filepath.Join(ctx.Application.Path, "bin", "app1"),
					},
					Direct:  true,
					Default: true,
				}))
			Expect(result.Processes).To(ContainElement(
				libcnb.Process{
					Type:    "app2",
					Command: "tini",
					Arguments: []string{
						"-g",
						"--",
						filepath.Join(ctx.Application.Path, "bin", "app2"),
					},
					Direct:  true,
					Default: false,
				}))
			Expect(result.Processes).To(ContainElement(
				libcnb.Process{
					Type:    "app3",
					Command: "tini",
					Arguments: []string{
						"-g",
						"--",
						filepath.Join(ctx.Application.Path, "bin", "app3"),
					},
					Direct:  true,
					Default: false,
				}))

			// TODO: BOM support isn't in yet
			// Expect(result.BOM.Entries).To(HaveLen(1))
			// Expect(result.BOM.Entries[0].Name).To(Equal("cargo"))
			// Expect(result.BOM.Entries[0].Build).To(BeTrue())
			// Expect(result.BOM.Entries[0].Launch).To(BeFalse())
		})

		context("BP_CARGO_TINI_DISABLED is true", func() {
			it.Before(func() {
				Expect(os.Setenv("BP_CARGO_TINI_DISABLED", "true")).To(Succeed())
			})

			it.After(func() {
				Expect(os.Unsetenv("BP_CARGO_TINI_DISABLED")).To(Succeed())
			})

			it("contributes cargo layer", func() {
				ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "rust-cargo"})

				service.On("ProjectTargets", mock.AnythingOfType("string")).Return([]string{"app1", "app2", "app3"}, nil)

				result, err := cargoBuild.Build(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.Layers).To(HaveLen(2))
				Expect(result.Layers[0].Name()).To(Equal("Cargo Cache"))
				Expect(result.Layers[1].Name()).To(Equal("Cargo"))

				Expect(result.Processes).To(HaveLen(3))
				Expect(result.Processes).To(ContainElement(
					libcnb.Process{
						Type:      "app1",
						Command:   filepath.Join(ctx.Application.Path, "bin", "app1"),
						Arguments: []string{},
						Direct:    true,
						Default:   true,
					}))
				Expect(result.Processes).To(ContainElement(
					libcnb.Process{
						Type:      "app2",
						Command:   filepath.Join(ctx.Application.Path, "bin", "app2"),
						Arguments: []string{},
						Direct:    true,
						Default:   false,
					}))
				Expect(result.Processes).To(ContainElement(
					libcnb.Process{
						Type:      "app3",
						Command:   filepath.Join(ctx.Application.Path, "bin", "app3"),
						Arguments: []string{},
						Direct:    true,
						Default:   false,
					}))

				// TODO: BOM support isn't in yet
				// Expect(result.BOM.Entries).To(HaveLen(1))
				// Expect(result.BOM.Entries[0].Name).To(Equal("cargo"))
				// Expect(result.BOM.Entries[0].Build).To(BeTrue())
				// Expect(result.BOM.Entries[0].Launch).To(BeFalse())
			})
		})
	})
}
