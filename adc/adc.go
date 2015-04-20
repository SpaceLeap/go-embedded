package adc

import (
	"fmt"
	"os"

	"github.com/SpaceLeap/go-embedded"
)

type Name string

const (
	AIN0 Name = "AIN0"
	AIN1 Name = "AIN1"
	AIN2 Name = "AIN2"
	AIN3 Name = "AIN3"
	AIN4 Name = "AIN4"
	AIN5 Name = "AIN5"
	AIN6 Name = "AIN6"
)

var (
	deviceTree string
	prefixDir  string
)

func Init(deviceTreePrefix string) error {
	err := embedded.LoadDeviceTree(deviceTreePrefix)
	if err != nil {
		return err
	}
	deviceTree = deviceTreePrefix

	ocpDir, err := embedded.BuildPath("/sys/devices", "ocp")
	if err != nil {
		return err
	}
	prefixDir, err = embedded.BuildPath(ocpDir, "helper")
	if err != nil {
		return err
	}
	prefixDir += "/AIN"
	return nil
}

func Cleanup() error {
	return embedded.UnloadDeviceTree(deviceTree)
}

type ADC struct {
	ain  Name
	file *os.File
}

func NewADC(ain Name) (*ADC, error) {
	filename := prefixDir + string(ain)
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	return &ADC{ain, file}, nil
}

func (adc *ADC) Close() error {
	return adc.file.Close()
}

func (adc *ADC) AIn() Name {
	return adc.ain
}

func (adc *ADC) ReadRaw() (value float32) {
	adc.file.Seek(0, os.SEEK_SET)
	fmt.Fscan(adc.file, &value)
	return value
}

func (adc *ADC) ReadValue() (value float32) {
	return adc.ReadRaw() / 1800.0
}
