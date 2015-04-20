package i2c

// #include <stddef.h>
// #include <sys/types.h>
// #include <linux/i2c-dev.h>
import "C"

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func SwapBytes(word uint16) uint16 {
	return word<<8 | word>>8
}

type Err struct {
	method string
	cause  error
}

func (err Err) Error() string {
	return fmt.Sprintf("I2C.%s error: %s", err.method, err.cause)
}

func wrapErr(method string, err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(Err); ok {
		e.method = method
		return e
	}
	return Err{method, err}
}

// I2C is a port of https://github.com/bivab/smbus-cffi/
type I2C struct {
	file    *os.File
	address int
}

// Connects the object to the specified SMBus.
func NewI2C(bus, address int) (*I2C, error) {
	filename := fmt.Sprintf("/dev/i2c-%d", bus)
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	i2c := &I2C{file: file, address: -1}
	err = i2c.SetAddress(address)
	if err != nil {
		file.Close()
		return nil, err
	}

	return i2c, nil
}

func (i2c *I2C) Close() error {
	return wrapErr("Close", i2c.file.Close())
}

func (i2c *I2C) Address() int {
	return i2c.address
}

func (i2c *I2C) SetAddress(address int) error {
	if address != i2c.address {
		result, _, errno := syscall.Syscall(syscall.SYS_IOCTL, i2c.file.Fd(), C.I2C_SLAVE, uintptr(address))
		if result != 0 {
			return Err{"SetAddress", errno}
		}
		i2c.address = address
	}
	return nil
}

func (i2c *I2C) smbusAccess(readWrite, register uint8, size int, data unsafe.Pointer) (uintptr, error) {
	args := C.struct_i2c_smbus_ioctl_data{
		read_write: C.char(readWrite),
		command:    C.__u8(register),
		size:       C.int(size),
		data:       (*C.union_i2c_smbus_data)(data),
	}
	result, _, errno := syscall.Syscall(syscall.SYS_IOCTL, i2c.file.Fd(), C.I2C_SMBUS, uintptr(unsafe.Pointer(&args)))
	if int(result) == -1 {
		return 0, errno
	}
	return result, nil
}

// WriteQuick sends a single bit to the device, at the place of the Rd/Wr bit.
func (i2c *I2C) WriteQuick(value uint8) error {
	_, err := i2c.smbusAccess(value, 0, C.I2C_SMBUS_QUICK, nil)
	return wrapErr("SetAddress", err)
}

// ReadUint8 reads a single byte from a device, without specifying a device
// register. Some devices are so simple that this interface is enough; for
// others, it is a shorthand if you want to read the same register as in
// the previous SMBus command.
func (i2c *I2C) ReadUint8() (result uint8, err error) {
	_, err = i2c.smbusAccess(C.I2C_SMBUS_READ, 0, C.I2C_SMBUS_BYTE, unsafe.Pointer(&result))
	if err != nil {
		return 0, wrapErr("ReadUint8", err)
	}
	return 0xFF & result, nil
}

// WriteUint8 sends a single byte to a device.
func (i2c *I2C) WriteUint8(value uint8) error {
	_, err := i2c.smbusAccess(C.I2C_SMBUS_WRITE, value, C.I2C_SMBUS_BYTE, nil)
	return wrapErr("WriteUint8", err)
}

// ReadInt8 reads a single byte from a device, without specifying a device
// register. Some devices are so simple that this interface is enough; for
// others, it is a shorthand if you want to read the same register as in
// the previous SMBus command.
func (i2c *I2C) ReadInt8() (int8, error) {
	result, err := i2c.ReadUint8()
	return int8(result), wrapErr("ReadInt8", err)
}

// WriteInt8 sends a single byte to a device.
func (i2c *I2C) WriteInt8(value int8) error {
	return wrapErr("WriteInt8", i2c.WriteUint8(uint8(value)))
}

// ReadUint8Reg reads a single byte from a device, from a designated register.
func (i2c *I2C) ReadUint8Reg(register uint8) (result uint8, err error) {
	_, err = i2c.smbusAccess(C.I2C_SMBUS_READ, register, C.I2C_SMBUS_BYTE_DATA, unsafe.Pointer(&result))
	if err != nil {
		return 0, wrapErr("ReadUint8Reg", err)
	}
	return 0xFF & result, nil
}

// WriteUint8Reg writes a single byte to a device, to a designated register.
func (i2c *I2C) WriteUint8Reg(register uint8, value uint8) error {
	_, err := i2c.smbusAccess(C.I2C_SMBUS_WRITE, register, C.I2C_SMBUS_BYTE_DATA, unsafe.Pointer(&value))
	return wrapErr("WriteUint8Reg", err)
}

// ReadInt8Reg reads a single byte from a device, from a designated register.
func (i2c *I2C) ReadInt8Reg(register uint8) (int8, error) {
	result, err := i2c.ReadUint8Reg(register)
	return int8(result), wrapErr("ReadInt8Reg", err)
}

// WriteInt8Reg writes a single byte to a device, to a designated register.
func (i2c *I2C) WriteInt8Reg(register uint8, value int8) error {
	return wrapErr("WriteInt8Reg", i2c.WriteUint8Reg(register, uint8(value)))
}

// ReadUint16Reg is very like ReadUint8Reg; again, data is read from a
// device, from a designated register.
// But this time, the data is a complete word (16 bits).
func (i2c *I2C) ReadUint16Reg(register uint8) (result uint16, err error) {
	_, err = i2c.smbusAccess(C.I2C_SMBUS_READ, register, C.I2C_SMBUS_WORD_DATA, unsafe.Pointer(&result))
	if err != nil {
		return 0, wrapErr("ReadUint16Reg", err)
	}
	return 0xFFFF & result, nil
}

// WriteUint16Reg is the opposite of the ReadUint16Reg operation. 16 bits
// of data is written to a device, to the designated register.
func (i2c *I2C) WriteUint16Reg(register uint8, value uint16) error {
	_, err := i2c.smbusAccess(C.I2C_SMBUS_WRITE, register, C.I2C_SMBUS_WORD_DATA, unsafe.Pointer(&value))
	return wrapErr("WriteUint16Reg", err)
}

// ReadUint16RegSwapped is very like ReadUint8Reg; again, data is read from a
// device, from a designated register. But this time, the data is a complete word (16 bits).
// The bytes of the 16 bit value will be swapped.
func (i2c *I2C) ReadUint16RegSwapped(register uint8) (result uint16, err error) {
	result, err = i2c.ReadUint16Reg(register)
	return SwapBytes(result), wrapErr("ReadUint16Reg", err)
}

// WriteUint16RegSwapped is the opposite of the ReadUint16RegSwapped operation. 16 bits
// of data is written to a device, to the designated register.
// The bytes of the 16 bit value will be swapped.
func (i2c *I2C) WriteUint16RegSwapped(register uint8, value uint16) error {
	return wrapErr("WriteUint16RegSwapped", i2c.WriteUint16Reg(register, SwapBytes(value)))
}

// ReadInt16Reg is very like ReadInt8Reg; again, data is read from a
// device, from a designated register. But this time, the data is a complete word (16 bits).
func (i2c *I2C) ReadInt16Reg(register uint8) (int16, error) {
	result, err := i2c.ReadUint16Reg(register)
	return int16(result), wrapErr("ReadInt16Reg", err)
}

// WriteInt16Reg is the opposite of the ReadInt16Reg operation. 16 bits
// of data is written to a device, to the designated register.
func (i2c *I2C) WriteInt16Reg(register uint8, value int16) error {
	return wrapErr("WriteInt16Reg", i2c.WriteUint16Reg(register, uint16(value)))
}

// ReadInt16RegSwapped is very like ReadInt8RegSwapped; again, data is read from a
// device, from a designated register. But this time, the data is a complete word (16 bits).
// The bytes of the 16 bit value will be swapped.
func (i2c *I2C) ReadInt16RegSwapped(register uint8) (int16, error) {
	result, err := i2c.ReadUint16RegSwapped(register)
	return int16(result), wrapErr("ReadInt16RegSwapped", err)
}

// WriteInt16RegSwapped is the opposite of the ReadInt16RegSwapped operation. 16 bits
// of data is written to a device, to the designated register.
// The bytes of the 16 bit value will be swapped.
func (i2c *I2C) WriteInt16RegSwapped(register uint8, value int16) error {
	return wrapErr("WriteInt16RegSwapped", i2c.WriteUint16RegSwapped(register, uint16(value)))
}

// ProcessCall selects a device register (through the register byte), sends
// 16 bits of data to it, and reads 16 bits of data in return.
func (i2c *I2C) ProcessCall(register uint8, value uint16) (uint16, error) {
	_, err := i2c.smbusAccess(C.I2C_SMBUS_WRITE, register, C.I2C_SMBUS_PROC_CALL, unsafe.Pointer(&value))
	if err != nil {
		return 0, wrapErr("ProcessCall", err)
	}
	return 0xFFFF & value, nil
}

// ProcessCallSwapped selects a device register (through the register byte), sends
// 16 bits of data to it, and reads 16 bits of data in return.
// The bytes of the 16 bit value will be swapped.
func (i2c *I2C) ProcessCallSwapped(register uint8, value uint16) (uint16, error) {
	result, err := i2c.ProcessCall(register, SwapBytes(value))
	return SwapBytes(result), wrapErr("ProcessCallSwapped", err)
}

// ProcessCallBlock reads a block of up to 32 bytes from a device, from a
// designated register.
func (i2c *I2C) ProcessCallBlock(register uint8, block []byte) ([]byte, error) {
	length := len(block)
	if length == 0 || length > C.I2C_SMBUS_BLOCK_MAX {
		return nil, wrapErr("ProcessCallBlock", fmt.Errorf("Length of block is %d, but must be in the range 1 to %d", length, C.I2C_SMBUS_BLOCK_MAX))
	}
	data := make([]byte, length+1, C.I2C_SMBUS_BLOCK_MAX+2)
	data[0] = byte(length)
	copy(data[1:], block)
	_, err := i2c.smbusAccess(C.I2C_SMBUS_WRITE, register, C.I2C_SMBUS_BLOCK_PROC_CALL, unsafe.Pointer(&data[0]))
	if err != nil {
		return nil, wrapErr("ProcessCallBlock", err)
	}
	return data[1 : 1+data[0]], nil
}

// ReadBlock writes up to 32 bytes to a device, to a designated register.
func (i2c *I2C) ReadBlock(register uint8) ([]byte, error) {
	data := make([]byte, C.I2C_SMBUS_BLOCK_MAX+2)
	_, err := i2c.smbusAccess(C.I2C_SMBUS_READ, register, C.I2C_SMBUS_BLOCK_DATA, unsafe.Pointer(&data[0]))
	if err != nil {
		return nil, wrapErr("ReadBlock", err)
	}
	return data[1 : 1+data[0]], nil
}

// WriteBlock selects a device register, sends
// 1 to 31 bytes of data to it, and reads 1 to 31 bytes of data in return.
func (i2c *I2C) WriteBlock(register uint8, block []byte) error {
	length := len(block)
	if length == 0 || length > C.I2C_SMBUS_BLOCK_MAX {
		return wrapErr("WriteBlock", fmt.Errorf("Length of block is %d, but must be in the range 1 to %d", length, C.I2C_SMBUS_BLOCK_MAX))
	}
	data := make([]byte, length+1)
	data[0] = byte(length)
	copy(data[1:], block)
	_, err := i2c.smbusAccess(C.I2C_SMBUS_WRITE, register, C.I2C_SMBUS_BLOCK_DATA, unsafe.Pointer(&data[0]))
	return wrapErr("WriteBlock", err)
}

// TODO: Perform I2C Block Read transaction.
// With if len == 32 then arg = C.I2C_SMBUS_I2C_BLOCK_BROKEN instead of I2C_SMBUS_I2C_BLOCK_DATA ???

func (i2c *I2C) Read(p []byte) (n int, err error) {
	n, err = i2c.file.Read(p)
	return n, wrapErr("Read", err)
}

func (i2c *I2C) Write(p []byte) (n int, err error) {
	n, err = i2c.file.Write(p)
	return n, wrapErr("Write", err)
}
