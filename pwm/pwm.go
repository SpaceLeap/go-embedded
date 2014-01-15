package pwm

import (
	"fmt"
	"os"
	"time"

	"github.com/SpaceLeap/go-embedded"
)

type Polarity uint

const (
	POLARITY_LOW  Polarity = 0
	POLARITY_HIGH Polarity = 1
)

type PWM struct {
	key          string
	period       time.Duration
	duty         time.Duration
	polarity     Polarity
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

func Cleanup() error {
	return embedded.UnloadDeviceTree(deviceTree)
}

func NewPWM(key string, period, duty time.Duration, polarity Polarity) (*PWM, error) {
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

	err = pwm.SetPolarity(polarity)
	if err != nil {
		pwm.Close()
		return nil, err
	}
	err = pwm.SetPeriod(period)
	if err != nil {
		pwm.Close()
		return nil, err
	}
	err = pwm.SetDuty(duty)
	if err != nil {
		pwm.Close()
		return nil, err
	}

	return pwm, nil
}

func (pwm *PWM) Key() string {
	return pwm.key
}

func (pwm *PWM) Period() time.Duration {
	return pwm.period
}

func (pwm *PWM) SetPeriod(period time.Duration) error {
	_, err := fmt.Fprintf(pwm.periodFile, "%d", period)
	if err != nil {
		return err
	}
	pwm.period = period
	return nil
}

func (pwm *PWM) Duty() time.Duration {
	return pwm.duty
}

func (pwm *PWM) SetDuty(duty time.Duration) error {
	_, err := fmt.Fprintf(pwm.dutyFile, "%d", duty)
	if err != nil {
		return err
	}
	pwm.duty = duty
	return nil
}

func (pwm *PWM) Polarity() Polarity {
	return pwm.polarity
}

func (pwm *PWM) SetPolarity(polarity Polarity) error {
	_, err := fmt.Fprintf(pwm.polarityFile, "%d", polarity)
	if err != nil {
		return err
	}
	pwm.polarity = polarity
	return nil
}

func (pwm *PWM) Close() error {
	pwm.periodFile.Close()
	pwm.dutyFile.Close()
	pwm.polarityFile.Close()
	return embedded.UnloadDeviceTree(devicePrefix + pwm.key)
}
