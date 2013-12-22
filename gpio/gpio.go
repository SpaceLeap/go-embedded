package gpio

import (
	"fmt"
	"os"
	"syscall"

	"github.com/ungerik/go-quick"
)

const (
	HIGH = true
	LOW  = false

	_EPOLLET = 1 << 31
)

type Edge string

const (
	NO_EDGE      Edge = "none"
	RISING_EDGE  Edge = "rising"
	FALLING_EDGE Edge = "falling"
	BOTH_EDGES   Edge = "both"
)

type Direction string

const (
	INPUT  Direction = "in"
	OUTPUT Direction = "out"
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
func NewGPIO(nr int) (*GPIO, error) {
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
	if Direction(direction) == OUTPUT {
		return OUTPUT, nil
	} else {
		return INPUT, nil
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

func (gpio *GPIO) openValueFile() error {
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

func (gpio *GPIO) Value() (bool, error) {
	if err := gpio.openValueFile(); err != nil {
		return false, err
	}
	gpio.value.Seek(0, os.SEEK_SET)
	val := make([]byte, 1)
	_, err := gpio.value.Read(val)
	if err != nil {
		return false, err
	}
	return val[0] == '1', nil
}

func (gpio *GPIO) SetValue(value bool) (err error) {
	if err = gpio.openValueFile(); err != nil {
		return err
	}
	gpio.value.Seek(0, os.SEEK_SET)
	if value {
		_, err = gpio.value.Write([]byte{'1'})
	} else {
		_, err = gpio.value.Write([]byte{'0'})
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

func (gpio *GPIO) AddEdgeDetect(edge Edge) (chan bool, error) {
	gpio.RemoveEdgeDetect()

	err := gpio.SetDirection(INPUT)
	if err != nil {
		return nil, err
	}
	err = gpio.SetEdge(edge)
	if err != nil {
		return nil, err
	}
	err = gpio.openValueFile()
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

	valueChan := make(chan bool)
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

func (gpio *GPIO) BlockingWaitForEdge(edge Edge) (value bool, err error) {
	valueChan, err := gpio.AddEdgeDetect(edge)
	if err == nil {
		value = <-valueChan
		gpio.RemoveEdgeDetect()
	}
	return value, err
}
