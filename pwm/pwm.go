package pwm

import (
	"fmt"
	"os"

	"github.com/SpaceLeap/go-embedded"
)

type PWM struct {
	key          string
	dutyCycle    float32
	frequencyGHz float32
	polarity     int
	periodFile   *os.File
	dutyFile     *os.File
	polarityFile *os.File
}

var (
	deviceTree   string
	devicePrefix string
)

func Init(deviceTreePrefix, pwmDevicePrefix string) error {
	err := embedded.LoadDeviceTree(deviceTreePrefix)
	if err != nil {
		return err
	}
	deviceTree = deviceTreePrefix
	devicePrefix = pwmDevicePrefix
	return nil
}

func NewPWM(key string, dutyCycle, frequencyGHz float32, polarity int) (*PWM, error) {
	err := embedded.LoadDeviceTree(devicePrefix + key)
	if err != nil {
		return nil, err
	}

	ocpDir, err := embedded.BuildPath("/sys/devices", "ocp")
	if err != nil {
		return nil, err
	}

	//finds and builds the pwmTestPath, as it can be variable...
	pwmTestPath, err := embedded.BuildPath(ocpDir, "pwm_test_"+key)
	if err != nil {
		return nil, err
	}

	//create the path for the period and duty
	periodPath := pwmTestPath + "/period"
	dutyPath := pwmTestPath + "/duty"
	polarityPath := pwmTestPath + "/polarity"

	periodFile, err := os.OpenFile(periodPath, os.O_RDWR, 0660)
	if err != nil {
		return nil, err
	}
	dutyFile, err := os.OpenFile(dutyPath, os.O_RDWR, 0660)
	if err != nil {
		periodFile.Close()
		return nil, err
	}
	polarityFile, err := os.OpenFile(polarityPath, os.O_RDWR, 0660)
	if err != nil {
		periodFile.Close()
		dutyFile.Close()
		return nil, err
	}

	pwm := &PWM{
		key:          key,
		periodFile:   periodFile,
		dutyFile:     dutyFile,
		polarityFile: polarityFile,
	}

	err = pwm.SetFrequency(frequencyGHz)
	if err != nil {
		pwm.Close()
		return nil, err
	}
	err = pwm.SetPolarity(polarity)
	if err != nil {
		pwm.Close()
		return nil, err
	}
	err = pwm.SetDutyCycle(dutyCycle)
	if err != nil {
		pwm.Close()
		return nil, err
	}

	return pwm, nil
}

func (pwm *PWM) Key() string {
	return pwm.key
}

// Frequency returns the signal frequency in Giga Hertz
func (pwm *PWM) Frequency() float32 {
	return pwm.frequencyGHz
}

// SetFrequency sets the signal frequency in Giga Hertz
func (pwm *PWM) SetFrequency(frequencyGHz float32) error {
	if frequencyGHz <= 0 {
		return fmt.Errorf("invalid frequency: %f", frequencyGHz)
	}

	periodNs := uint(1e9 / frequencyGHz)
	_, err := fmt.Fprintf(pwm.periodFile, "%d", periodNs)
	if err != nil {
		return err
	}

	pwm.frequencyGHz = frequencyGHz
	return nil
}

func (pwm *PWM) Polarity() int {
	return pwm.polarity
}

func (pwm *PWM) SetPolarity(polarity int) error {
	if polarity < 0 || polarity > 1 {
		return fmt.Errorf("polarity must be either 0 or 1")
	}

	_, err := fmt.Fprintf(pwm.polarityFile, "%d", polarity)
	if err != nil {
		return err
	}

	pwm.polarity = polarity
	return nil
}

// DutyCycle returns the duty cicly of the signal with range from 0.0 to 1.0.
func (pwm *PWM) DutyCycle() float32 {
	return pwm.dutyCycle
}

// SetDutyCycle sets the duty cicly of the signal.
// dutyCycle must be in the range from 0.0 to 1.0
func (pwm *PWM) SetDutyCycle(dutyCycle float32) error {
	if dutyCycle < 0 || dutyCycle > 1 {
		return fmt.Errorf("dutyCycle %f not in range 0.0 to 1.0", dutyCycle)
	}

	periodNs := 1e9 / pwm.frequencyGHz
	duty := uint(periodNs * dutyCycle)
	_, err := fmt.Fprintf(pwm.dutyFile, "%d", duty)
	if err != nil {
		return err
	}

	pwm.dutyCycle = dutyCycle
	return nil
}

func (pwm *PWM) Close() {
	embedded.UnloadDeviceTree(devicePrefix + pwm.key)
	pwm.periodFile.Close()
	pwm.dutyFile.Close()
	pwm.polarityFile.Close()
}

func CleanupPWM() error {
	return embedded.UnloadDeviceTree(deviceTree)
}
