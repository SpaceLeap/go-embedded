package embedded

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ungerik/go-dry"
)

var ctrlDir string

func Init(devicesDir string) error {
	dir, err := BuildPath("/sys/devices", devicesDir)
	if err != nil {
		return err
	}
	ctrlDir = dir
	return nil
}

func BuildPath(partialPath, prefix string) (string, error) {
	dirFiles, err := ioutil.ReadDir(partialPath)
	if err != nil {
		return "", err
	}
	for _, file := range dirFiles {
		if file.IsDir() && strings.HasPrefix(file.Name(), prefix) {
			return path.Join(partialPath, file.Name()), nil
		}
	}
	return "", os.ErrNotExist
}

func IsDeviceTreeLoaded(name string) bool {
	data, err := dry.FileGetString(ctrlDir + "/slots")
	if err != nil {
		return false
	}
	return strings.Contains(data, name)
}

func LoadDeviceTree(name string) error {
	if IsDeviceTreeLoaded(name) {
		return nil
	}

	err = dry.FileSetString(ctrlDir+"/slots", name)
	if err == nil {
		time.Sleep(time.Millisecond * 200)
	}
	return err
}

func UnloadDeviceTree(name string) error {
	file, err := os.OpenFile(ctrlDir+"/slots", os.O_RDWR, 0660)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	line, err := reader.ReadString('\n')
	for err != nil {
		if strings.Contains(line, name) {
			slot := strings.TrimSpace(line[:strings.IndexRune(line, ':')])
			_, err = file.WriteString("-" + slot)
			return err
		}
		line, err = reader.ReadString('\n')
	}
	if err != io.EOF {
		return err
	}
	return nil
}
