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
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-community/cargo/cargo"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx        libcnb.BuildContext
		cargoBuild cargo.Build
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "build-application")
		Expect(err).NotTo(HaveOccurred())

		// ctx.Buildpack.Metadata = map[string]interface{}{
		// 	"configurations": []map[string]interface{}{
		// 		{"name": "BP_MAVEN_BUILD_ARGUMENTS", "default": "test-argument"},
		// 	},
		// }

		ctx.Layers.Path, err = ioutil.TempDir("", "build-layers")
		Expect(err).NotTo(HaveOccurred())

		cargoBuild = cargo.Build{
			Logger: bard.NewLogger(&bytes.Buffer{}),
		}
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

	it("contributes cargo layer", func() {
		ctx.Plan.Entries = append(ctx.Plan.Entries, libcnb.BuildpackPlanEntry{Name: "rust-cargo"})

		result, err := cargoBuild.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(HaveLen(2))
		Expect(result.Layers[0].Name()).To(Equal("Cargo Cache"))
		Expect(result.Layers[1].Name()).To(Equal("Cargo"))

		// TODO: BOM support isn't in yet
		// Expect(result.BOM.Entries).To(HaveLen(1))
		// Expect(result.BOM.Entries[0].Name).To(Equal("cargo"))
		// Expect(result.BOM.Entries[0].Build).To(BeTrue())
		// Expect(result.BOM.Entries[0].Launch).To(BeFalse())
	})

}
