package mtimes_test

import (
	"bytes"
	"fmt"
	"github.com/onsi/gomega/types"
	"github.com/paketo-buildpacks/packit/scribe"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dmikusa/rust-cargo-cnb/mtimes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testMTimes(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect  = NewWithT(t).Expect
		workDir string
	)

	it.Before(func() {
		var err error

		workDir, err = ioutil.TempDir("", "mtimes-test")
		Expect(err).NotTo(HaveOccurred())

		// set up expected files
		Expect(touch(filepath.Join(workDir, "testdata/folder1/file1a.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/folder1/file1b.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/folder1/folder2/file2a.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/folder1/folder2/file2b.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/folder1/folder2/folder3/file3a.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/folder1/folder2/folder3/file3b.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/foldera/filea1.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/foldera/filea2.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/foldera/folderb/fileb1.txt"))).ToNot(HaveOccurred())
		Expect(touch(filepath.Join(workDir, "testdata/foldera/folderb/folderc/filec1.txt"))).ToNot(HaveOccurred())

		// modify the times to ensure an expected output from mtimes module (git does not preserver mtimes)
		Expect(changeTime(filepath.Join(workDir, "testdata"), "2021-04-13T21:32:42.266625461")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1"), "2021-04-13T21:32:16.56220856")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1/file1a.txt"), "2021-04-13T21:32:11.619000841")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1/file1b.txt"), "2021-04-13T21:32:16.562185855")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1/folder2"), "2021-04-13T21:33:12.836115365")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1/folder2/file2a.txt"), "2021-04-13T21:32:24.132")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1/folder2/file2b.txt"), "2021-04-13T21:33:12.836100946")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1/folder2/folder3"), "2021-04-13T21:33:24.454010729")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1/folder2/folder3/file3a.txt"), "2021-04-13T21:33:21.115193516")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/folder1/folder2/folder3/file3b.txt"), "2021-04-17T22:39:03.01803")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/foldera"), "2021-04-13T21:31:49.712696151")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/foldera/filea1.txt"), "2021-04-13T21:31:44.719991295")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/foldera/filea2.txt"), "2021-04-13T21:31:49.712681414")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/foldera/folderb"), "2021-04-13T21:31:36.645595542")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/foldera/folderb/fileb1.txt"), "2021-04-13T21:31:36.645581261")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/foldera/folderb/folderc"), "2021-04-13T21:31:27.300292894")).ToNot(HaveOccurred())
		Expect(changeTime(filepath.Join(workDir, "testdata/foldera/folderb/folderc/filec1.txt"), "2021-04-13T21:31:27.30028183")).ToNot(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(workDir)).To(Succeed())
	})

	context("mtimes walks the tree", func() {
		it("saves the directory state", func() {
			logs := bytes.Buffer{}

			// change one file's time on-the-fly to ensure we're getting accurate results
			currentTime := time.Now().UTC()
			checkTimePath := filepath.Join(workDir, "testdata/folder1/folder2/folder3/file3b.txt")
			err := os.Chtimes(checkTimePath, currentTime, currentTime)
			Expect(err).ToNot(HaveOccurred())

			err = mtimes.NewPreserver(scribe.NewEmitter(&logs)).Preserve(filepath.Join(workDir, "testdata"))
			Expect(err).ToNot(HaveOccurred())
			mtimesFile := filepath.Join(workDir, "testdata/mtimes.json")
			Expect(mtimesFile).To(BeARegularFile())

			buf, err := ioutil.ReadFile(mtimesFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(buf)).To(ContainSubstring("testdata"))
			Expect(string(buf)).To(ContainSubstring("testdata/folder1"))
			Expect(string(buf)).To(ContainSubstring("testdata/folder1/folder2"))
			Expect(string(buf)).To(ContainSubstring("testdata/folder1/folder2/file2a.txt"))
			Expect(string(buf)).To(ContainSubstring("testdata/foldera/folderb/folderc/filec1.txt"))
			Expect(string(buf)).To(ContainSubstring(
				fmt.Sprintf(`"MTime":"%s"`, currentTime.Format("2006-01-02T15:04:05.999999999Z"))))
		})

		it("restores the directory state", func() {
			logs := bytes.Buffer{}

			// wipe all the mtimes
			originTime := time.Unix(0, 0).UTC()
			Expect(filepath.WalkDir(filepath.Join(workDir, "testdata"), func(path string, d fs.DirEntry, err error) error {
				Expect(err).ToNot(HaveOccurred())
				err = os.Chtimes(path, originTime, originTime)
				Expect(err).ToNot(HaveOccurred())
				return nil
			})).ToNot(HaveOccurred())
			Expect(filepath.Join(workDir, "testdata", "folder1")).To(HaveMTime(originTime))

			// copy test mtimes.json file
			data, err := ioutil.ReadFile("testdata/mtimes.json")
			Expect(err).ToNot(HaveOccurred())
			data = []byte(strings.ReplaceAll(string(data), "##workdir##", workDir))
			err = ioutil.WriteFile(filepath.Join(workDir, "testdata/mtimes.json"), data, 0644)
			Expect(err).ToNot(HaveOccurred())

			preserver := mtimes.NewPreserver(scribe.NewEmitter(&logs))
			Expect(preserver.Restore(filepath.Join(workDir, "testdata"))).ToNot(HaveOccurred())
			Expect(filepath.Join(workDir, "testdata/folder1")).To(HaveMTime("2021-04-13T21:32:16.56220856"))
			Expect(filepath.Join(workDir, "testdata/folder1/file1a.txt")).To(HaveMTime("2021-04-13T21:32:11.619000841"))
			Expect(filepath.Join(workDir, "testdata/folder1/folder2/folder3/file3a.txt")).To(HaveMTime("2021-04-13T21:33:21.115193516"))
			Expect(filepath.Join(workDir, "testdata/foldera/folderb")).To(HaveMTime("2021-04-13T21:31:36.645595542"))
		})
	})
}

func HaveMTime(expected interface{}) types.GomegaMatcher {
	expectedTime, ok := expected.(time.Time)
	if !ok {
		expectedTimeStr, ok := expected.(string)
		if !ok {
			panic(fmt.Errorf("unable to determine time, requires time.Time or string in format `2006-01-02T15:04:05.999999999` UTC"))
		}

		var err error
		expectedTime, err = time.Parse("2006-01-02T15:04:05.999999999", expectedTimeStr)
		if err != nil {
			panic(fmt.Errorf("unable to determine time, requires time.Time or string in format `2006-01-02T15:04:05.999999999` UTC\n%w", err))
		}
	}
	return &haveMTimeMatcher{expected: expectedTime}
}

type haveMTimeMatcher struct {
	expected time.Time
	actual   time.Time
}

func (matcher *haveMTimeMatcher) Match(actual interface{}) (success bool, err error) {
	filePath, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("HaveMTime expects a string file path")
	}

	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return false, fmt.Errorf("unable to read file info %s\n%w", filePath, err)
	}
	matcher.actual = fileInfo.ModTime().UTC()

	return matcher.expected == matcher.actual, nil
}

func (matcher *haveMTimeMatcher) FailureMessage(_ interface{}) (message string) {
	return fmt.Sprintf("Expected %s to equal %s", matcher.actual, matcher.expected)
}

func (matcher *haveMTimeMatcher) NegatedFailureMessage(_ interface{}) (message string) {
	return fmt.Sprintf("Expected %s not to equal %s", matcher.actual, matcher.expected)
}

func touch(fileName string) error {
	dirName := filepath.Dir(fileName)
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		return fmt.Errorf("unable to create directory\n%w", err)
	}

	_, err = os.Stat(fileName)
	if os.IsNotExist(err) {
		file, err := os.Create(fileName)
		if err != nil {
			return fmt.Errorf("unable to create %s\n%w", fileName, err)
		}
		err = file.Close()
		if err != nil {
			return fmt.Errorf("unable to close %s\n%w", fileName, err)
		}
	}

	return nil
}

func changeTime(fileName string, toTime string) error {
	bumpTime, err := time.Parse("2006-01-02T15:04:05.999999999", toTime)
	if err != nil {
		return fmt.Errorf("unable to parse time %s, use format `2006-01-02T15:04:05.999999999` UTC\n%w", toTime, err)
	}
	err = os.Chtimes(fileName, bumpTime, bumpTime)
	if err != nil {
		return fmt.Errorf("unable to change file time\n%w", err)
	}

	return nil
}
