package lps25

import (
	"fmt"

	"github.com/ardnew/mcp2221a"
)

func hasBit(b byte, pos uint8) bool {
	return b&(1<<pos) != 0
}

func clearBit(b byte, pos uint8) byte {
	return b & (0xFF - (1 << pos))
}

func setBit(b byte, pos uint8) byte {
	return b | (1 << pos)
}

func setBitTo(b byte, pos uint8, high uint8) byte {
	if high == 0 {
		return clearBit(b, pos)
	} else {
		return setBit(b, pos)
	}
}

const (
	DefaultI2CAddress = 0x5D

	ControlRegister1 = 0x20
	EnabledBit       = 7
	ODR0Bit          = 4
	ODR1Bit          = 5
	ODR2Bit          = 6

	ControlRegister2 = 0x21
	SwResetBit       = 2

	PressureOutXLRegister = 0x28
)

type Sensor struct {
	i2c     *mcp2221a.I2C
	address uint8
}

func NewSensor(i2c *mcp2221a.I2C, address uint8) *Sensor {
	if address == 0 {
		address = DefaultI2CAddress
	}

	return &Sensor{
		i2c:     i2c,
		address: address,
	}
}

func (s Sensor) readRegister(reg uint8) (byte, error) {
	buf, err := s.i2c.ReadReg(s.address, reg, 1)
	if err != nil {
		return 0, err
	}

	return buf[0], nil
}

func (s Sensor) Enabled() (bool, error) {
	reg, err := s.readRegister(ControlRegister1)
	if err != nil {
		return false, err
	}

	return hasBit(reg, EnabledBit), nil
}

func (s Sensor) patchRegister(register uint8, patch func(byte) byte) error {
	reg, err := s.readRegister(register)
	if err != nil {
		return fmt.Errorf("failed to read current register: %w", err)
	}

	err = s.i2c.Write(true, s.address, []byte{register, patch(reg)}, 2)
	if err != nil {
		return fmt.Errorf("failed to write updated register: %w", err)
	}

	return nil
}

func (s Sensor) patchRegisterBit(register uint8, bit uint8, patch func(bool) bool) error {
	return s.patchRegister(register, func(b byte) byte {
		if patch(hasBit(b, bit)) {
			return setBit(b, bit)
		}

		return clearBit(b, bit)
	})
}

func (s Sensor) Enable() error {
	return s.patchRegisterBit(ControlRegister1, EnabledBit, func(b bool) bool {
		return true
	})
}

func (s Sensor) Disable() error {
	return s.patchRegisterBit(ControlRegister1, EnabledBit, func(b bool) bool {
		return false
	})
}

func (s Sensor) Reset() error {
	return s.patchRegisterBit(ControlRegister2, SwResetBit, func(b bool) bool {
		return true
	})
}

type DataRate int

const (
	DataRateOneShot DataRate = iota
	DataRate1Hz
	DataRate7Hz
	DataRate12_5Hz
	DataRate25Hz
)

func (s Sensor) SetDataRate(rate DataRate) error {
	return s.patchRegister(ControlRegister1, func(b byte) byte {
		var ord0, ord1, ord2 uint8

		switch rate {
		case DataRateOneShot:
			ord0, ord1, ord2 = 0, 0, 0
		case DataRate1Hz:
			ord0, ord1, ord2 = 1, 0, 0
		case DataRate7Hz:
			ord0, ord1, ord2 = 0, 1, 0
		case DataRate12_5Hz:
			ord0, ord1, ord2 = 1, 1, 0
		case DataRate25Hz:
			ord0, ord1, ord2 = 0, 0, 1
		}

		b = setBitTo(b, ODR0Bit, ord0)
		b = setBitTo(b, ODR1Bit, ord1)
		b = setBitTo(b, ODR2Bit, ord2)

		return b
	})
}

func (s Sensor) Pressure() (float32, error) {
	// | 0x80 to enable auto-incrementing addresses while reading the 3 bytes
	reg := byte(PressureOutXLRegister | 0x80)

	if err := s.i2c.Write(false, s.address, []byte{reg}, 1); err != nil {
		return 0, fmt.Errorf("failed to init: %w", err)
	}

	data, err := s.i2c.Read(true, s.address, 3)
	if err != nil {
		return 0, fmt.Errorf("failed to read data: %w", err)
	}

	added := uint32(data[2])<<16 | uint32(data[1])<<8 | uint32(data[0])

	return float32(added) / 4096.0, nil
}
