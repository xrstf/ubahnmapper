package main

import (
	"fmt"
	"log"
	"time"

	mcp "github.com/ardnew/mcp2221a"
	"go.xrstf.de/ubahnmapper/pkg/lps25"
)

func main() {
	m, err := mcp.New(0, mcp.VID, mcp.PID)
	if err != nil {
		log.Fatalf("Failed to open mcp2221a device: %v", err)
	}
	defer m.Close()

	// reset device to default settings stored in flash memory
	// if err := m.Reset(5 * time.Second); err != nil {
	// 	log.Fatalf("Failed to reset device: %v", err)
	// }

	// configure I2C module to use default baud rate (optional)
	if err := m.I2C.SetConfig(mcp.I2CBaudRate); err != nil {
		log.Fatalf("Failed to setup I²C bus: %v", err)
	}

	sensor := lps25.NewSensor(m.I2C, 0) // 0 = default address

	enabled, err := sensor.Enabled()
	if err != nil {
		log.Fatalf("Failed to get sensor status: %v", err)
	}

	if !enabled {
		err = sensor.Enable()
		if err != nil {
			log.Fatalf("Failed to enable sensor: %v", err)
		}
	}

	err = sensor.SetDataRate(lps25.DataRate25Hz)
	if err != nil {
		log.Fatalf("Failed to set sensor data rate: %v", err)
	}

	for {
		pressure, err := sensor.Pressure()
		if err != nil {
			log.Fatalf("Failed to read pressure: %v", err)
		}

		fmt.Printf("pressure: %f hPa\n", pressure)
		time.Sleep(1 * time.Second)
	}
}
