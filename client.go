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
	"time"

	"github.com/tarm/serial"
)

// Arduino Firmata client for golang
type Client struct {
	dev  string
	baud int
	conn io.ReadWriteCloser

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
}

// NewClient creates a new Client and connects to the Arduino board
// over specified serial port. It blocks till a connection is
// succesfully established and pin mappings are retrieved.
func NewClient(dev string, baud int) (*Client, error) {
	c := &serial.Config{Name: dev, Baud: baud}
	conn, err := serial.OpenPort(c)
	if err != nil {
		return nil, err
	}

	client := &Client{
		dev:       dev,
		baud:      baud,
		conn:      conn,
		valueChan: make(chan FirmataValue),
	}

	inited := client.replyReader()
	conn.Write([]byte{byte(SystemReset)})

	for {
		select {
		case <-inited:
			return client, err
		case <-time.After(time.Second * 15):
			conn.Write([]byte{byte(SystemReset)})
		case <-time.After(time.Second * 30):
			conn.Close()
			break
		}
	}

	return nil, errors.New("cannot open connection to the device; timeout")
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// SetPinMode sets the pin mode.
func (c *Client) SetPinMode(pin uint8, mode PinMode) error {
	if c.pinModes[pin][mode] == nil {
		return fmt.Errorf("pin mode = %v not supported by pin %v", mode, pin)
	}
	return c.sendCommand([]byte{byte(SetPinMode), (pin & 0x7F), byte(mode)})
}

// Specified if a digital Pin should be watched for input.
// Values will be streamed back over a channel which can be retrieved by the GetValues() call
func (c *Client) EnableDigitalInput(pin uint, val bool) error {
	if pin < 0 || pin > uint(len(c.pinModes)) {
		return fmt.Errorf("invalid pin number: %v", pin)
	}
	port := (pin / 8) & 0x7F
	pin = pin % 8

	if val {
		cmd := []byte{byte(EnableDigitalInput) | byte(port), 0x01}
		return c.sendCommand(cmd)
	}
	cmd := []byte{byte(EnableDigitalInput) | byte(port), 0x00}
	return c.sendCommand(cmd)
}

// Set the value of a digital pin
func (c *Client) DigitalWrite(pin uint8, val bool) error {
	if pin < 0 || pin > uint8(len(c.pinModes)) && c.pinModes[pin][Output] != nil {
		return fmt.Errorf("invalid pin number: %v", pin)
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
	return c.sendCommand(cmd)
}

// Specified if a analog Pin should be watched for input.
// Values will be streamed back over a channel which can be retrieved by the Values() call.
func (c *Client) EnableAnalogInput(pin uint, val bool) error {
	if pin < 0 || pin > uint(len(c.pinModes)) && c.pinModes[pin][Analog] != nil {
		return fmt.Errorf("invalid pin number: %v\n", pin)
	}
	ch := byte(c.analogPinsChannelMap[int(pin)])
	if val {
		cmd := []byte{byte(EnableAnalogInput) | ch, 0x01}
		return c.sendCommand(cmd)
	}
	cmd := []byte{byte(EnableAnalogInput) | ch, 0x00}
	return c.sendCommand(cmd)
}

func (c *Client) AnalogWrite(pin uint, pinData byte) error {
	if pin < 0 || pin > uint(len(c.pinModes)) && c.pinModes[pin][Analog] != nil {
		return fmt.Errorf("invalid pin number %v\n", pin)
	}
	data := to7Bit(pinData)
	cmd := []byte{byte(AnalogMessage) | byte(pin), data[0], data[1]}
	return c.sendCommand(cmd)
}

func (c *Client) sendCommand(cmd []byte) error {
	// TODO(jbd): Do not concat.
	bStr := ""
	for _, b := range cmd {
		bStr = bStr + fmt.Sprintf(" %#2x", b)
	}
	_, err := c.conn.Write(cmd)
	return err
}

func (c *Client) SetAnalogSamplingInterval(ms byte) error {
	data := to7Bit(ms)
	return c.sendSysEx(SamplingInterval, data[0], data[1])
}

func (c *Client) Values() <-chan FirmataValue {
	return c.valueChan
}
