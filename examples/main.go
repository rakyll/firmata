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

	myDelay := time.Millisecond * 250

	arduino.SetPinMode(led, firmata.Output)
	for x := 0; x < 10; x++ {
		// Turn ON led
		arduino.DigitalWrite(led, true)
		time.Sleep(myDelay)
		// Turn OFF led
		arduino.DigitalWrite(led, false)
		time.Sleep(myDelay)

	}
	arduino.Close()
}
