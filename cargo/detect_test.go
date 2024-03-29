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
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-community/cargo/cargo"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx    libcnb.DetectContext
		detect cargo.Detect
	)

	it.Before(func() {
		var err error

		ctx.Application.Path = t.TempDir()
		Expect(err).ToNot(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
	})

	context("missing required files", func() {
		it("missing Cargo.toml", func() {
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "Cargo.lock"), []byte{}, 0644))

			plan, err := detect.Detect(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).To(Equal(libcnb.DetectResult{}))
		})

		it("missing Cargo.lock", func() {
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "Cargo.toml"), []byte{}, 0644))

			plan, err := detect.Detect(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).To(Equal(libcnb.DetectResult{}))
		})

		it("missing both Cargo.toml and Cargo.lock", func() {
			plan, err := detect.Detect(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(plan).To(Equal(libcnb.DetectResult{}))
		})
	})

	it("passes with both Cargo.toml and Cargo.lock", func() {
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "Cargo.toml"), []byte{}, 0644))
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "Cargo.lock"), []byte{}, 0644))

		Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{
			Pass: true,
			Plans: []libcnb.BuildPlan{
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: "rust-cargo"},
					},
					Requires: []libcnb.BuildPlanRequire{
						{Name: "syft"},
						{Name: "rust-cargo"},
						{Name: "rust"},
					},
				},
			},
		}))
	})
}
