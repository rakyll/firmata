// Copyright 2014 Krishna Raman
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package firmata

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/tarm/serial"
)

// Arduino Firmata client for golang
type FirmataClient struct {
	serialDev string
	baud      int
	conn      io.ReadWriteCloser
	Log       *log.Logger

	protocolVersion []byte
	firmwareVersion []int
	firmwareName    string

	analogMappingDone bool
	capabilityDone    bool

	digitalPinState [8]byte

	analogPinsChannelMap map[int]byte
	analogChannelPinsMap map[byte]int
	pinModes             []map[PinMode]interface{}

	valueChan  chan FirmataValue
	serialChan chan string
	spiChan    chan []byte

	Verbose bool
}

// Creates a new FirmataClient object and connects to the Arduino board
// over specified serial port. This function blocks till a connection is
// succesfullt established and pin mappings are retrieved.
func NewClient(dev string, baud int) (client *FirmataClient, err error) {
	c := &serial.Config{Name: dev, Baud: baud}
	conn, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}

	client = &FirmataClient{
		serialDev: dev,
		baud:      baud,
		conn:      conn,
		Log:       log.New(os.Stdout, "[go-firmata] ", log.Ltime),
	}

	done := client.replyReader()
	conn.Write([]byte{byte(SystemReset)})

	for {
		select {
		case <-done:
			return client, err
		case <-time.After(time.Second * 15):
			conn.Write([]byte{byte(SystemReset)})
		case <-time.After(time.Second * 30):
			conn.Close()
			return nil, errors.New("cannot open connection to the device; timeout")
		}
	}
	return
}

// Close the serial connection to properly clean up after ourselves
// Usage: defer client.Close()
func (c *FirmataClient) Close() error {
	return c.conn.Close()
}

// Sets the Pin mode (input, output, etc.) for the Arduino pin
func (c *FirmataClient) SetPinMode(pin uint8, mode PinMode) error {
	if c.pinModes[pin][mode] == nil {
		return fmt.Errorf("Pin mode %v not supported by pin %v", mode, pin)
	}
	cmd := []byte{byte(SetPinMode), (pin & 0x7F), byte(mode)}
	if err := c.sendCommand(cmd); err != nil {
		return err
	}
	c.Log.Printf("SetPinMode: pin %d -> %s\r\n", pin, mode)
	return nil
}

// Specified if a digital Pin should be watched for input.
// Values will be streamed back over a channel which can be retrieved by the GetValues() call
func (c *FirmataClient) EnableDigitalInput(pin uint, val bool) (err error) {
	if pin < 0 || pin > uint(len(c.pinModes)) {
		err = fmt.Errorf("Invalid pin number %v\n", pin)
		return
	}
	port := (pin / 8) & 0x7F
	pin = pin % 8

	if val {
		cmd := []byte{byte(EnableDigitalInput) | byte(port), 0x01}
		err = c.sendCommand(cmd)
	} else {
		cmd := []byte{byte(EnableDigitalInput) | byte(port), 0x00}
		err = c.sendCommand(cmd)
	}

	return
}

// Set the value of a digital pin
func (c *FirmataClient) DigitalWrite(pin uint8, val bool) error {
	if pin < 0 || pin > uint8(len(c.pinModes)) && c.pinModes[pin][Output] != nil {
		return fmt.Errorf("Invalid pin number %v\n", pin)
	}
	port := (pin / 8) & 0x7F
	portData := &c.digitalPinState[port]
	pin = pin % 8
	if val {
		(*portData) = (*portData) | (1 << pin)
	} else {
		(*portData) = (*portData) & ^(1 << pin)
	}
	data := to7Bit(*(portData))
	cmd := []byte{byte(DigitalMessage) | byte(port), data[0], data[1]}
	if err := c.sendCommand(cmd); err != nil {
		return err
	}
	c.Log.Printf("DigitalWrite: pin %d -> %t\r\n", pin, val)
	return nil
}

// Specified if a analog Pin should be watched for input.
// Values will be streamed back over a channel which can be retrieved by the GetValues() call
func (c *FirmataClient) EnableAnalogInput(pin uint, val bool) (err error) {
	if pin < 0 || pin > uint(len(c.pinModes)) && c.pinModes[pin][Analog] != nil {
		err = fmt.Errorf("Invalid pin number %v\n", pin)
		return
	}

	ch := byte(c.analogPinsChannelMap[int(pin)])
	c.Log.Printf("Enable analog inout on pin %v channel %v", pin, ch)
	if val {
		cmd := []byte{byte(EnableAnalogInput) | ch, 0x01}
		err = c.sendCommand(cmd)
	} else {
		cmd := []byte{byte(EnableAnalogInput) | ch, 0x00}
		err = c.sendCommand(cmd)
	}

	return
}

// Set the value of a analog pin
func (c *FirmataClient) AnalogWrite(pin uint, pinData byte) error {
	if pin < 0 || pin > uint(len(c.pinModes)) && c.pinModes[pin][Analog] != nil {
		return fmt.Errorf("Invalid pin number %v\n", pin)
	}
	data := to7Bit(pinData)
	cmd := []byte{byte(AnalogMessage) | byte(pin), data[0], data[1]}
	return c.sendCommand(cmd)
}

func (c *FirmataClient) sendCommand(cmd []byte) error {
	// TODO(jbd): Do not concat.
	bStr := ""
	for _, b := range cmd {
		bStr = bStr + fmt.Sprintf(" %#2x", b)
	}
	_, err = c.conn.Write(cmd)
	return err
}

// Sets the polling interval in milliseconds for analog pin samples
func (c *FirmataClient) SetAnalogSamplingInterval(ms byte) (err error) {
	data := to7Bit(ms)
	err = c.sendSysEx(SamplingInterval, data[0], data[1])
	return
}

// Get the channel to retrieve analog and digital pin values
func (c *FirmataClient) GetValues() <-chan FirmataValue {
	return c.valueChan
}
