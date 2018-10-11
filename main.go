package main

import (
	"fmt"
	"os"
	"time"

	"github.com/stianeikeland/go-rpio"
)

type LightCycle struct {
	name              string
	pin               rpio.Pin
	riseHour, riseMin int
	setHour, setMin   int
	Active            bool
}

func (cycle *LightCycle) Name() string {
	return cycle.name
}

func (cycle *LightCycle) Pin() rpio.Pin {
	return cycle.pin
}

func (cycle *LightCycle) RiseClock() (hour, min int) {
	return cycle.riseHour, cycle.riseMin
}

func (cycle *LightCycle) SetClock() (hour, min int) {
	return cycle.setHour, cycle.setMin
}

var (
	// Use mcu pin 4, corresponds to physical pin 7 on the pi
	solenoid = rpio.Pin(4)

	// Use mcu pin 17, corresponds to physical pin 11 on the pi
	lightGroup1 = rpio.Pin(17)
)

func virtualRain() {
	for {
		solenoid.Low()
		fmt.Println("Water valve open...")
		time.Sleep(time.Second * 5)
		solenoid.High()
		fmt.Println("Water valve close...")
		time.Sleep(time.Minute * 5)
	}
}

func virtualSun(cycles []*LightCycle) {
	//http://hortsci.ashspublications.org/content/28/5/552.6
	/*
		Abstract
		--------
			Most works on artificial lighting of winter greenhouse vegetable crops studied the effects of
		photosynthetic photon flux but rarely photoperiod. Over the last three years, we conducted experiments to find
		out the best photoperiods for production of greenhouse tomato and pepper. We found that extending photoperiod
		up to 20 hrs increased productivity of pepper plants while continuous light (24 hrs) decreased yields. For
		tomato plants, productivity reached a maximum under a 14-hr photoperiod while longer photoperiods (16 to 24
		hrs) did not increase yields. For both pepper and tomato plants, optimal growth (shoot fresh and dry weights)
		was obtained under the same photoperiods that gave the best productivities. We also observed leaf chloroses on
		tomato plants after 6 weeks under photoperiods of 20 and 24 hrs and leaf deformations (wrinkles) on pepper
		plants exposed to continuous lighting. We also observed that plants under continuous light grew better and
		flowered earlier during the first 5 to 7 weeks of treatments. So, tomato and pepper plants can use
		advantageously continuous supplemental lighting for a short period of time but are negatively affected on a
		long term basis. Future works should look at varying photoperiods to optimize yields.
	*/

	//Set them all to off to start
	for _, cycle := range cycles {
		cycle.Pin().High()
		cycle.Active = false
	}

	for {
		hour, min, _ := time.Now().Clock()
		fmt.Printf("%d:%d\n", hour, min)
		for _, cycle := range cycles {
			riseHour, riseMin := cycle.RiseClock()
			setHour, setMin := cycle.SetClock()
			if hour >= riseHour && min >= riseMin && hour < setHour {
				if !cycle.Active {
					cycle.Pin().Low()
					fmt.Println(cycle.Name(), "- Sun up!")
					cycle.Active = true
				}
			} else if (hour >= setHour && min >= setMin) || hour < riseHour {
				if cycle.Active {
					cycle.Pin().High()
					fmt.Println(cycle.Name(), "- Sunset!")
					cycle.Active = false
				}
			}
		}
		time.Sleep(time.Minute)
	}
}

func main() {
	// Open and map memory to access gpio, check for errors
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Unmap gpio memory when done
	defer rpio.Close()

	// Set pin to output mode
	solenoid.Output()
	lightGroup1.Output()

	cycles := make([]*LightCycle, 1)
	cycles[0] = &LightCycle{"MainSun", lightGroup1, 6, 0, 22, 0, false} //Main sun rises at 6:00 and sets at 22:00

	go virtualRain()
	virtualSun(cycles)
}
