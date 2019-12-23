package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

func main() {
	root := os.Args[1]
	imageList := makeImageList(root, env2fileList(root, os.Getenv("IGNORED_FILES")))
	if len(imageList) == 0 {
		fmt.Println("there is no image files")
		os.Exit(0)
	}

	pullRequestMessage := []string{
		"[imgcmp] Optimize images",
		"",
		"## Successfully optimized",
	}
	reportTable := []string{
		"<details>",
		"",
		"<summary>details</summary>",
		"",
		"|File Name|Before|After|Diff (size)|Diff (rate)|",
		"|:---|---:|---:|---:|---:|",
	}
	var totalBeforeSize int64 = 0
	var totalAfterSize int64 = 0
	wg := &sync.WaitGroup{}
	mutex := &sync.Mutex{}
	// optimize images and make reportTable
	for _, path := range imageList {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			beforeSize := fileSize(p)
			optimizeImage(p)
			afterSize := fileSize(p)
			mutex.Lock()
			reportTable = append(reportTable, tableRow(p, beforeSize, afterSize))
			mutex.Unlock()
			totalBeforeSize += beforeSize
			totalAfterSize += afterSize
		}(path)
	}
	wg.Wait()

	// pull request
	if (totalAfterSize - totalBeforeSize) == 0 {
		fmt.Println("images are already optimized")
		os.Exit(0)
	} else {
		reportTable = append(reportTable, tableRow("Total", totalBeforeSize, totalAfterSize), "", "</details>")
		pullRequestMessage = append(
			pullRequestMessage,
			fmt.Sprintf(
				"Your image files have been optimized (File size: **%v** (**%v**))!",
				byte2unit(totalAfterSize-totalBeforeSize),
				diffRate(totalBeforeSize, totalAfterSize),
			),
			"",
		)
		file, err := os.Create("./pull_request_message.md")
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(0)
		}
		defer file.Close()

		pullRequestMessage = append(pullRequestMessage, reportTable...)
		output := strings.Join(pullRequestMessage, "\n")
		file.Write(([]byte)(output))

		fmt.Println("Successfully optimized")
	}
}

func env2fileList(root string, env string) []string {
	fileList := []string{}
	fileListWildcard := strings.Split(env, ":")
	if env == "" {
		return fileList
	}
	for _, fileWildcard := range fileListWildcard {
		files, _ := filepath.Glob(root + "/" + fileWildcard)
		fileList = append(fileList, files...)
	}
	return fileList
}

func makeImageList(root string, ignoredFileList []string) []string {
	ignoredRegexp := regexp.MustCompile(strings.Join(ignoredFileList, "|"))
	if len(ignoredFileList) == 0 {
		// 0^ matches nothing
		ignoredRegexp = regexp.MustCompile("0^")
	}

	skipDirRegexp := regexp.MustCompile(`^\..+`)
	imageList := []string{}
	callback := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if skipDirRegexp.MatchString(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		fileType := strings.Split(execCommand("file", []string{path}), " ")[1]

		if !ignoredRegexp.MatchString(path) && (isJPEG(fileType) || isPNG(fileType) || isGIF(fileType) || isSVG(fileType)) {
			imageList = append(imageList, path)
		}
		return nil
	}
	err := filepath.Walk(root, callback)
	if err != nil {
		fmt.Println(1, err)
	}
	return imageList
}

func isJPEG(fileType string) bool {
	return fileType == "JPEG"
}

func isPNG(fileType string) bool {
	return fileType == "PNG"
}

func isGIF(fileType string) bool {
	return fileType == "GIF"
}

func isSVG(fileType string) bool {
	return fileType == "SVG"
}

func optimizeImage(path string) {
	fileType := strings.Split(execCommand("file", []string{path}), " ")[1]
	if isJPEG(fileType) {
		execCommand("jpegoptim", []string{"-m85", path})
	} else if isPNG(fileType) {
		execCommand("optipng", []string{"-o2", path})
	} else if isGIF(fileType) {
		execCommand("gifsicle", []string{"-b", "-O3", "--colors", "256", path})
	} else if isSVG(fileType) {
		execCommand("svgo", []string{path})
	}
}

func execCommand(command string, args []string) string {
	output, err := exec.Command(command, args...).Output()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	return string(output)
}

func fileSize(path string) int64 {
	file, errOpen := os.Open(path)
	if errOpen != nil {
		fmt.Println(errOpen.Error())
	}
	info, errStat := file.Stat()
	if errStat != nil {
		fmt.Println(errStat.Error())
	}
	return info.Size()
}

func byte2unit(size int64) string {
	var res string
	negative := size < 0
	size = int64(math.Abs(float64(size)))
	if size >= 1e9 {
		res = fmt.Sprintf("%.2f GB", float64(size)/1e9)
	} else if size >= 1e6 {
		res = fmt.Sprintf("%.2f MB", float64(size)/1e6)
	} else if size >= 1e3 {
		res = fmt.Sprintf("%.2f kB", float64(size)/1e3)
	} else {
		res = fmt.Sprintf("%v Byte", size)
	}
	if negative {
		res = "-" + res
	}
	return res
}

func diffRate(before int64, after int64) string {
	return fmt.Sprintf("%.2f", float64(after-before)/float64(before)*100) + "%"
}

func tableRow(name string, before int64, after int64) string {
	return fmt.Sprintf("|%v|%v|%v|%v|%v|", name, byte2unit(before), byte2unit(after), byte2unit(after-before), diffRate(before, after))
}
