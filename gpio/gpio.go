package gpio

import (
	"fmt"
	"os"
	"syscall"

	"github.com/ungerik/go-quick"
)

type Value int

const (
	LOW  Value = 0
	HIGH Value = 1

	_EPOLLET = 1 << 31
)

type Edge string

const (
	EDGE_NONE    Edge = "none"
	EDGE_RISING  Edge = "rising"
	EDGE_FALLING Edge = "falling"
	EDGE_BOTH    Edge = "both"
)

type Direction string

const (
	DIRECTION_IN  Direction = "in"
	DIRECTION_OUT Direction = "out"
	// ALT0   Direction = 4
)

// TODO: How is this configured?
// Maybe see https://github.com/adafruit/PyBBIO/blob/master/bbio/bbio.py
type PullUpDown int

const (
	PUD_OFF  PullUpDown = 0
	PUD_DOWN PullUpDown = 1
	PUD_UP   PullUpDown = 2
)

type GPIO struct {
	nr    int
	value *os.File
	epfd  quick.SyncInt
}

// NewGPIO exports the GPIO pin nr.
func NewGPIO(nr int, direction ...Direction) (*GPIO, error) {
	gpio := &GPIO{nr: nr}

	export, err := os.OpenFile("/sys/class/gpio/export", os.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer export.Close()

	_, err = fmt.Fprintf(export, "%d", gpio.nr)
	if err != nil {
		return nil, err
	}

	if len(direction) > 0 {
		err = gpio.SetDirection(direction[0])
		if err != nil {
			return nil, err
		}
	}

	return gpio, nil
}

// Close unexports the GPIO pin.
func (gpio *GPIO) Close() error {
	gpio.RemoveEdgeDetect()

	if gpio.value != nil {
		gpio.value.Close()
	}

	unexport, err := os.OpenFile("/sys/class/gpio/unexport", os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer unexport.Close()

	_, err = fmt.Fprintf(unexport, "%d", gpio.nr)
	return err
}

func (gpio *GPIO) Direction() (Direction, error) {
	filename := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", gpio.nr)
	file, err := os.OpenFile(filename, os.O_RDONLY|syscall.O_NONBLOCK, 0666)
	if err != nil {
		return "", err
	}
	defer file.Close()
	direction := make([]byte, 3)
	_, err = file.Read(direction)
	if err != nil {
		return "", err
	}
	if Direction(direction) == DIRECTION_OUT {
		return DIRECTION_OUT, nil
	} else {
		return DIRECTION_IN, nil
	}
}

func (gpio *GPIO) SetDirection(direction Direction) error {
	filename := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", gpio.nr)
	file, err := os.OpenFile(filename, os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write([]byte(direction))
	return err
}

func (gpio *GPIO) ensureValueFileIsOpen() error {
	if gpio.value != nil {
		return nil
	}
	filename := fmt.Sprintf("/sys/class/gpio/gpio%d/value", gpio.nr)
	file, err := os.OpenFile(filename, os.O_RDWR, 0666)
	if err == nil {
		gpio.value = file
	}
	return err
}

func (gpio *GPIO) Value() (Value, error) {
	if err := gpio.ensureValueFileIsOpen(); err != nil {
		return 0, err
	}
	gpio.value.Seek(0, os.SEEK_SET)
	val := make([]byte, 1)
	_, err := gpio.value.Read(val)
	if err != nil {
		return 0, err
	}
	if val[0] == '0' {
		return LOW, nil
	} else {
		return HIGH, nil
	}
}

func (gpio *GPIO) SetValue(value Value) (err error) {
	if err = gpio.ensureValueFileIsOpen(); err != nil {
		return err
	}
	gpio.value.Seek(0, os.SEEK_SET)
	if value == 0 {
		_, err = gpio.value.Write([]byte{'0'})
	} else {
		_, err = gpio.value.Write([]byte{'1'})
	}
	return err
}

func (gpio *GPIO) SetEdge(edge Edge) error {
	filename := fmt.Sprintf("/sys/class/gpio/gpio%d/edge", gpio.nr)
	file, err := os.OpenFile(filename, os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write([]byte(edge))
	return err
}

func (gpio *GPIO) AddEdgeDetect(edge Edge) (chan Value, error) {
	gpio.RemoveEdgeDetect()

	err := gpio.SetDirection(DIRECTION_IN)
	if err != nil {
		return nil, err
	}
	err = gpio.SetEdge(edge)
	if err != nil {
		return nil, err
	}
	err = gpio.ensureValueFileIsOpen()
	if err != nil {
		return nil, err
	}

	epfd, err := syscall.EpollCreate(1)
	if err != nil {
		return nil, err
	}

	event := &syscall.EpollEvent{
		Events: syscall.EPOLLIN | _EPOLLET | syscall.EPOLLPRI,
		Fd:     int32(gpio.value.Fd()),
	}
	err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, int(gpio.value.Fd()), event)
	if err != nil {
		syscall.Close(epfd)
		return nil, err
	}

	// / first time triggers with current state, so ignore
	_, err = syscall.EpollWait(epfd, make([]syscall.EpollEvent, 1), -1)
	if err != nil {
		syscall.Close(epfd)
		return nil, err
	}

	gpio.epfd.Set(epfd)

	valueChan := make(chan Value)
	go func() {
		for gpio.epfd.Get() != 0 {
			n, _ := syscall.EpollWait(epfd, make([]syscall.EpollEvent, 1), -1)
			if n > 0 {
				value, err := gpio.Value()
				if err == nil {
					valueChan <- value
				}
			}
		}
	}()
	return valueChan, nil
}

func (gpio *GPIO) RemoveEdgeDetect() {
	epfd := gpio.epfd.Swap(0)
	if epfd != 0 {
		syscall.Close(epfd)
	}
}

func (gpio *GPIO) BlockingWaitForEdge(edge Edge) (value Value, err error) {
	valueChan, err := gpio.AddEdgeDetect(edge)
	if err == nil {
		value = <-valueChan
		gpio.RemoveEdgeDetect()
	}
	return value, err
}
