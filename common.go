package embedded

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ungerik/go-quick"
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

func LoadDeviceTree(name string) error {
	slots := ctrlDir + "/slots"

	data, err := quick.FileGetString(slots)
	if err != nil {
		return err
	}

	if strings.Contains(data, name) {
		return nil
	}

	err = quick.FileSetString(slots, name)
	if err == nil {
		time.Sleep(time.Millisecond * 200)
	}
	return err
}

func UnloadDeviceTree(name string) error {
	slots := ctrlDir + "/slots"

	file, err := os.OpenFile(slots, os.O_RDWR, 0660)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	line, err := reader.ReadString('\n')
	for err != nil {
		if strings.Contains(line, name) {
			line = line[:strings.IndexRune(line, ':')]
			line = strings.TrimSpace(line)
			_, err = file.WriteString("-" + line)
			return err
		}
		line, err = reader.ReadString('\n')
	}
	if err != io.EOF {
		return err
	}
	return nil
}
