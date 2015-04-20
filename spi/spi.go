package spi

// #include <linux/spi/spidev.h>
// #include <sys/ioctl.h>
import "C"

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/SpaceLeap/go-embedded"
)

const (
	CPHA       uint8 = 0x01 /* clock phase */
	CPOL       uint8 = 0x02 /* clock polarity */
	CS_HIGH    uint8 = 0x04 /* chipselect active high? */
	LSB_FIRST  uint8 = 0x08 /* per-word bits-on-wire */
	THREE_WIRE uint8 = 0x10 /* SI/SO signals shared */
	LOOP       uint8 = 0x20 /* loopback mode */
)

// not used yet
const (
	NO_CS   = 0x40  /* 1 dev/bus, no chipselect */
	READY   = 0x80  /* slave pulls low to pause */
	TX_DUAL = 0x100 /* transmit with 2 wires */
	TX_QUAD = 0x200 /* transmit with 4 wires */
	RX_DUAL = 0x400 /* receive with 2 wires */
	RX_QUAD = 0x800 /* receive with 4 wires */
)

// SPI mode as two bit pattern of
// Clock Polarity and Phase [CPOL|CPHA]
// min: 0b00 = 0 max: 0b11 = 3
type Mode uint8

const (
	MODE_0 Mode = 0 /* (original MicroWire) */
	MODE_1 Mode = Mode(CPHA)
	MODE_2 Mode = Mode(CPOL)
	MODE_3 Mode = Mode(CPOL | CPHA)
)

var deviceTreePrefix string

func Init(deviceTree string) {
	deviceTreePrefix = deviceTree
}

type SPI struct {
	bus         int
	device      int
	file        *os.File /* open file descriptor: /dev/spi-X.Y */
	mode        uint8    /* current SPI mode */
	bitsPerWord uint8    /* current SPI bits per word setting */
	maxSpeedHz  uint32   /* current SPI max speed setting in Hz */
}

// NewSPI returns a new SPI object that is connected to the
// specified SPI device interface.
//
// NewSPI(X,Y) will open /dev/spidev-X.Y
//
// SPI is an object type that allows SPI transactions
// on hosts running the Linux kernel. The host kernel must have SPI
// support and SPI device interface support.
// All of these can be either built-in to the kernel, or loaded from modules.
//
// Because the SPI device interface is opened R/W, users of this
// module usually must have root permissions.
func NewSPI(bus, device int) (spi *SPI, err error) {
	err = embedded.LoadDeviceTree(fmt.Sprintf("%s%d", deviceTreePrefix, bus))
	if err != nil {
		return nil, err
	}

	spi = &SPI{bus: bus, device: device}

	path := fmt.Sprintf("/dev/spidev%d.%d", bus+1, device)
	spi.file, err = os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	r, _, err := syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_RD_MODE, uintptr(unsafe.Pointer(&spi.mode)))
	if r != 0 {
		return nil, err
	}

	r, _, err = syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_RD_BITS_PER_WORD, uintptr(unsafe.Pointer(&spi.bitsPerWord)))
	if r != 0 {
		return nil, err
	}

	r, _, err = syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_RD_MAX_SPEED_HZ, uintptr(unsafe.Pointer(&spi.maxSpeedHz)))
	if r != 0 {
		return nil, err
	}

	return spi, nil
}

// Disconnects the object from the interface.
func (spi *SPI) Close() error {
	return spi.file.Close()
}

// Read len(data) bytes from SPI device.
func (spi *SPI) Read(data []byte) (n int, err error) {
	return spi.file.Read(data)
}

// Write data to SPI device.
func (spi *SPI) Write(data []byte) (n int, err error) {
	return spi.file.Write(data)
}

type spi_ioc_transfer struct {
	tx_buf        uintptr
	rx_buf        uintptr
	len           uint32
	speed_hz      uint32
	delay_usecs   uint16
	bits_per_word uint8
	cs_change     uint8
	pad           uint32
}

// Xfer performs a SPI transaction.
// CS will be released and reactivated between blocks.
// delay specifies delay in usec between blocks.
func (spi *SPI) Xfer(txBuf []byte, delay_usecs uint16) (rxBuf []byte, err error) {
	length := len(txBuf)
	rxBuf = make([]byte, length)

	xfer := make([]spi_ioc_transfer, length)
	for i := range xfer {
		xfer[i].tx_buf = uintptr(unsafe.Pointer(&txBuf[i]))
		xfer[i].rx_buf = uintptr(unsafe.Pointer(&rxBuf[i]))
		xfer[i].len = 1
		xfer[i].delay_usecs = delay_usecs
	}

	SPI_IOC_MESSAGE := C._IOC_WRITE<<C._IOC_DIRSHIFT | C.SPI_IOC_MAGIC<<C._IOC_TYPESHIFT | length<<C._IOC_SIZESHIFT

	r, _, err := syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), uintptr(SPI_IOC_MESSAGE), uintptr(unsafe.Pointer(&xfer[0])))
	if r != 0 {
		return nil, err
	}

	// WA:
	// in CS_HIGH mode CS isn't pulled to low after transfer, but after read
	// reading 0 bytes doesnt matter but brings cs down
	syscall.Syscall(syscall.SYS_READ, spi.file.Fd(), uintptr(unsafe.Pointer(&rxBuf[0])), 0)

	return rxBuf, nil
}

// Xfer2 performs a SPI transaction.
// CS will be held active between blocks.
func (spi *SPI) Xfer2(txBuf []byte, delay_usecs uint16) (rxBuf []byte, err error) {
	length := len(txBuf)
	rxBuf = make([]byte, length)

	xfer := spi_ioc_transfer{
		tx_buf: uintptr(unsafe.Pointer(&txBuf[0])),
		rx_buf: uintptr(unsafe.Pointer(&rxBuf[0])),
		len:    uint32(length),
	}

	SPI_IOC_MESSAGE := C._IOC_WRITE<<C._IOC_DIRSHIFT | C.SPI_IOC_MAGIC<<C._IOC_TYPESHIFT | 1<<C._IOC_SIZESHIFT

	r, _, err := syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), uintptr(SPI_IOC_MESSAGE), uintptr(unsafe.Pointer(&xfer)))
	if r != 0 {
		return nil, err
	}

	// WA:
	// in CS_HIGH mode CS isn't pulled to low after transfer, but after read
	// reading 0 bytes doesnt matter but brings cs down
	syscall.Syscall(syscall.SYS_READ, spi.file.Fd(), uintptr(unsafe.Pointer(&rxBuf[0])), 0)

	return rxBuf, nil
}

func (spi *SPI) Mode() Mode {
	return Mode(spi.mode) & MODE_3
}

func (spi *SPI) SetMode(mode Mode) error {
	newMode := (spi.mode &^ uint8(MODE_3)) | uint8(mode)
	err := spi.setModeInt(newMode)
	if err == nil {
		spi.mode = newMode
	}
	return err
}

// CS active high
func (spi *SPI) CSHigh() bool {
	return spi.mode&CS_HIGH != 0
}

// CS active high
func (spi *SPI) SetCSHigh(csHigh bool) error {
	return spi.setModeFlag(csHigh, CS_HIGH)
}

func (spi *SPI) LSBFirst() bool {
	return spi.mode&LSB_FIRST != 0
}

func (spi *SPI) SetLSBFirst(lsbFirst bool) error {
	return spi.setModeFlag(lsbFirst, LSB_FIRST)
}

func (spi *SPI) ThreeWire() bool {
	return spi.mode&THREE_WIRE != 0
}

func (spi *SPI) SetThreeWire(threeWire bool) error {
	return spi.setModeFlag(threeWire, THREE_WIRE)
}

// Loop returns the loopback configuration.
func (spi *SPI) Loop() bool {
	return spi.mode&THREE_WIRE != 0
}

// SetLoop sets the loopback configuration.
func (spi *SPI) SetLoop(loop bool) error {
	return spi.setModeFlag(loop, LOOP)
}

func (spi *SPI) BitsPerWord() uint8 {
	return spi.bitsPerWord
}

func (spi *SPI) SetBitsPerWord(bits uint8) error {
	if bits < 8 || bits > 16 {
		return fmt.Errorf("SPI bits per word %d outside of valid range 8 to 16", bits)
	}

	r, _, err := syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_WR_BITS_PER_WORD, uintptr(unsafe.Pointer(&bits)))
	if r != 0 {
		return err
	}

	var test uint8
	r, _, err = syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_RD_BITS_PER_WORD, uintptr(unsafe.Pointer(&test)))
	if r != 0 {
		return err
	}

	if test == bits {
		spi.bitsPerWord = bits
		return nil
	} else {
		return fmt.Errorf("Could not set SPI bits per word %d", bits)
	}
}

func (spi *SPI) MaxSpeedHz() uint32 {
	return spi.maxSpeedHz
}

func (spi *SPI) SetMaxSpeedHz(maxSpeedHz uint32) error {
	r, _, err := syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_WR_MAX_SPEED_HZ, uintptr(unsafe.Pointer(&maxSpeedHz)))
	if r != 0 {
		return err
	}

	var test uint32
	r, _, err = syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_RD_MAX_SPEED_HZ, uintptr(unsafe.Pointer(&test)))
	if r != 0 {
		return err
	}

	if test == maxSpeedHz {
		spi.maxSpeedHz = maxSpeedHz
		return nil
	} else {
		return fmt.Errorf("Could not set SPI max speed in hz %d", maxSpeedHz)
	}
}

func (spi *SPI) setModeFlag(flag bool, mask uint8) error {
	newMode := spi.mode
	if flag {
		newMode |= mask
	} else {
		newMode &= ^mask
	}
	err := spi.setModeInt(newMode)
	if err == nil {
		spi.mode = newMode
	}
	return err
}

func (spi *SPI) setModeInt(mode uint8) error {
	r, _, err := syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_WR_MODE, uintptr(unsafe.Pointer(&mode)))
	if r != 0 {
		return err
	}

	var test uint8
	r, _, err = syscall.Syscall(syscall.SYS_IOCTL, spi.file.Fd(), C.SPI_IOC_RD_MODE, uintptr(unsafe.Pointer(&test)))
	if r != 0 {
		return err
	}

	if test == mode {
		return nil
	} else {
		return fmt.Errorf("Could not set SPI mode %X", mode)
	}
}
