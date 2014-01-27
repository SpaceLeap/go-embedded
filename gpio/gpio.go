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
	nr        int
	valueFile *os.File
	epollFd   quick.SyncInt
	edge      Edge
}

// NewGPIO exports the GPIO pin nr.
func NewGPIO(nr int, direction Direction) (*GPIO, error) {
	gpio := &GPIO{nr: nr}

	err := quick.FilePrintf("/sys/class/gpio/export", "%d", gpio.nr)
	if err != nil {
		return nil, err
	}

	err = gpio.SetDirection(direction)
	if err != nil {
		return nil, err
	}

	return gpio, nil
}

// Close unexports the GPIO pin.
func (gpio *GPIO) Close() error {
	gpio.DisableEdgeDetect()

	if gpio.valueFile != nil {
		gpio.valueFile.Close()
	}

	return quick.FilePrintf("/sys/class/gpio/unexport", "%d", gpio.nr)
}

func (gpio *GPIO) Direction() (Direction, error) {
	filename := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", gpio.nr)
	direction, err := quick.FileGetString(filename)
	return Direction(direction), err
}

func (gpio *GPIO) SetDirection(direction Direction) error {
	filename := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", gpio.nr)
	return quick.FileSetString(filename, string(direction))
}

// func (gpio *GPIO) SetPullUpDown(pull PullUpDown) error {
// 	file, err := os.OpenFile("/sys/kernel/debug/omap_mux/", os.O_WRONLY, 0660)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()
// 	_, err = file.Write([]byte(fmt.Sprintf("%X", 0x07|1<<5|pull)))
// 	return err
// }

func (gpio *GPIO) ensureValueFileIsOpen() error {
	if gpio.valueFile != nil {
		return nil
	}
	filename := fmt.Sprintf("/sys/class/gpio/gpio%d/value", gpio.nr)
	file, err := os.OpenFile(filename, os.O_RDWR, 0660)
	if err == nil {
		gpio.valueFile = file
	}
	return err
}

func (gpio *GPIO) Value() (Value, error) {
	if err := gpio.ensureValueFileIsOpen(); err != nil {
		return 0, err
	}
	gpio.valueFile.Seek(0, os.SEEK_SET)
	val := make([]byte, 1)
	_, err := gpio.valueFile.Read(val)
	if err != nil {
		return 0, err
	}
	return Value(val[0] - '0'), nil
}

func (gpio *GPIO) SetValue(value Value) (err error) {
	if err = gpio.ensureValueFileIsOpen(); err != nil {
		return err
	}
	gpio.valueFile.Seek(0, os.SEEK_SET)
	_, err = gpio.valueFile.Write([]byte{'0' + byte(value)})
	return err
}

func (gpio *GPIO) setEdge(edge Edge) error {
	if edge == gpio.edge {
		return nil
	}
	filename := fmt.Sprintf("/sys/class/gpio/gpio%d/edge", gpio.nr)
	return quick.FileSetString(filename, string(edge))
}

// func (gpio *GPIO) EdgeDetectCallback(edge Edge, callback func(Value)) error {
// 	gpio.DisableEdgeDetect()

// 	err := gpio.SetDirection(DIRECTION_IN)
// 	if err != nil {
// 		return err
// 	}
// 	err = gpio.SetEdge(edge)
// 	if err != nil {
// 		return err
// 	}
// 	err = gpio.ensureValueFileIsOpen()
// 	if err != nil {
// 		return err
// 	}

// 	epollFd, err := syscall.EpollCreate(1)
// 	if err != nil {
// 		return err
// 	}

// 	event := &syscall.EpollEvent{
// 		Events: syscall.EPOLLIN | _EPOLLET | syscall.EPOLLPRI,
// 		Fd:     int32(gpio.valueFile.Fd()),
// 	}
// 	err = syscall.EpollCtl(epollFd, syscall.EPOLL_CTL_ADD, int(gpio.valueFile.Fd()), event)
// 	if err != nil {
// 		syscall.Close(epollFd)
// 		return err
// 	}

// 	// / first time triggers with current state, so ignore
// 	_, err = syscall.EpollWait(epollFd, make([]syscall.EpollEvent, 1), -1)
// 	if err != nil {
// 		syscall.Close(epollFd)
// 		return err
// 	}

// 	gpio.epollFd.Set(epollFd)

// 	go func() {
// 		for gpio.epollFd.Get() != 0 {
// 			n, _ := syscall.EpollWait(epollFd, make([]syscall.EpollEvent, 1), -1)
// 			if n > 0 {
// 				value, err := gpio.Value()
// 				if err == nil {
// 					callback(value)
// 				}
// 			}
// 		}
// 	}()

// 	return nil
// }

// func (gpio *GPIO) EdgeDetect(edge Edge) (chan Value, error) {
// 	valueChan := make(chan Value)
// 	err := gpio.EdgeDetectCallback(edge, func(value Value) {
// 		valueChan <- value
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return valueChan, nil
// }

func (gpio *GPIO) WaitForEdge(edge Edge) (value Value, err error) {
	// valueChan, err := gpio.EdgeDetect(edge)
	// if err == nil {
	// 	value = <-valueChan
	// 	gpio.DisableEdgeDetect()
	// }
	// return value, err

	err = gpio.setEdge(edge)
	if err != nil {
		return 0, err
	}

	err = gpio.ensureValueFileIsOpen()
	if err != nil {
		return 0, err
	}

	epollFd := gpio.epollFd.Get()
	if epollFd == 0 {
		epollFd, err = syscall.EpollCreate(1)
		if err != nil {
			return 0, err
		}

		event := &syscall.EpollEvent{
			Events: syscall.EPOLLIN | _EPOLLET | syscall.EPOLLPRI,
			Fd:     int32(gpio.valueFile.Fd()),
		}
		err = syscall.EpollCtl(epollFd, syscall.EPOLL_CTL_ADD, int(gpio.valueFile.Fd()), event)
		if err != nil {
			syscall.Close(epollFd)
			return 0, err
		}

		// first time triggers with current state, so ignore
		_, err = syscall.EpollWait(epollFd, make([]syscall.EpollEvent, 1), -1)
		if err != nil {
			syscall.Close(epollFd)
			return 0, err
		}

		gpio.epollFd.Set(epollFd)
	}

	_, err = syscall.EpollWait(epollFd, make([]syscall.EpollEvent, 1), -1)
	if err != nil {
		return 0, err
	}
	return gpio.Value()
}

func (gpio *GPIO) IsEdgeDetectEnabled() bool {
	return gpio.epollFd.Get() != 0
}

func (gpio *GPIO) DisableEdgeDetect() {
	epollFd := gpio.epollFd.Swap(0)
	if epollFd != 0 {
		syscall.Close(epollFd)
	}
	gpio.setEdge(EDGE_NONE)
}
