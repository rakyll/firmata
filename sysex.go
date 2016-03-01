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
	"bytes"
	"fmt"
)

func (c *FirmataClient) parseSysEx(data []byte) {
	cmd := SysExCommand(data[0])
	data = data[1:]

	bStr := ""
	for _, b := range data {
		bStr = bStr + fmt.Sprintf(" %#2x", b)
	}

	switch {
	case cmd == StringData:
		// do nothing?
	case cmd == CapabilityResponse:
		dataBuf := bytes.NewBuffer(data)
		c.pinModes = make([]map[PinMode]interface{}, 0)

		pin := 0
		var err error
		var modes []byte
		for ; err == nil; modes, err = dataBuf.ReadBytes(127) {
			pinModes := make(map[PinMode]interface{})
			if len(modes) < 2 {
				continue
			}

			modes = modes[0 : len(modes)-1]
			for i := 0; i < len(modes); i = i + 2 {
				pinModes[PinMode(modes[i])] = modes[i+1]
			}
			c.pinModes = append(c.pinModes, pinModes)
			pin = pin + 1
		}
		c.capabilityDone = true
	case cmd == AnalogMappingResponse:
		c.analogPinsChannelMap = make(map[int]byte)
		c.analogChannelPinsMap = make(map[byte]int)
		for pin, channel := range data {
			if channel != 127 {
				c.analogPinsChannelMap[pin] = channel
				c.analogChannelPinsMap[channel] = pin
			}
		}
		c.analogMappingDone = true
	case cmd == ReportFirmware:
		c.firmwareVersion = make([]int, 2)
		c.firmwareVersion[0] = int(data[0])
		c.firmwareVersion[1] = int(data[1])
		data = data[2:]
		c.firmwareName = multibyteString(data)
		c.sendSysEx(AnalogMappingQuery)
		c.sendSysEx(CapabilityQuery)
	case cmd == Serial:
		c.parseSerialResponse(data)
	case cmd == SysExSPI:
		c.parseSPIResponse(data)
	}
}

func (c *FirmataClient) sendSysEx(cmd SysExCommand, data ...byte) (err error) {
	var b bytes.Buffer
	b.WriteByte(byte(StartSysEx))
	b.WriteByte(byte(cmd))
	b.Write(data)
	b.WriteByte(byte(EndSysEx))

	bStr := ""
	for _, b := range b.Bytes() {
		bStr = bStr + fmt.Sprintf(" %#2x", b)
	}
	_, err = b.WriteTo(c.conn)
	return
}
