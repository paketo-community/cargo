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
	. "github.com/onsi/gomega"
	"github.com/paketo-community/cargo/cargo"
	"github.com/sclevine/spec"
)

func testCache(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx    libcnb.BuildContext
		appDir string
	)

	it.Before(func() {
		var err error

		appDir = t.TempDir()
		Expect(err).NotTo(HaveOccurred())

		ctx.Layers.Path = t.TempDir()
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(appDir)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("symlinks to destination if it does not exist", func() {
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = cargo.Cache{AppPath: appDir}.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Cache).To(BeTrue())

		targetPath := filepath.Join(appDir, "target")
		fi, err := os.Lstat(targetPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(fi.Mode() & os.ModeSymlink).To(Equal(os.ModeSymlink))

		Expect(os.Readlink(targetPath)).To(Equal(layer.Path))
	})
}
