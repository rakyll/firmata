package main

import (
	"time"

	"github.com/rakyll/go-firmata"
)

var led uint8 = 13

func main() {
	arduino, err := firmata.NewClient("/dev/cu.usbmodem1421", 57600)
	if err != nil {
		panic(err)
	}
	arduino.SetPinMode(led, firmata.Output)
	for {
		// Turn ON led
		arduino.DigitalWrite(led, true)
		time.Sleep(time.Millisecond * 250)
		// Turn OFF led
		arduino.DigitalWrite(led, false)
		time.Sleep(time.Millisecond * 250)
	}
}
