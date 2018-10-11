package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/stianeikeland/go-rpio"

	"github.com/julienschmidt/httprouter"
	"net/http"
)

type LightCycle struct {
	name              string
	pin               rpio.Pin
	riseHour, riseMin int
	setHour, setMin   int
	Active            bool
	ForceOn           bool //Force the lights to stay on
	ForceOff          bool //Force the lights to stay off
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
	//Override for the water
	solenoidForceClosed = false //Force the water to stop running
	solenoidForceOpen   = false //Force the water to keep running
	solenoidOpen        = false //State tracking

	// Use mcu pin 4, corresponds to physical pin 7 on the pi
	solenoid = rpio.Pin(4)

	// Use mcu pin 17, corresponds to physical pin 11 on the pi
	lightGroup1 = rpio.Pin(17)

	//Light cycles
	mainCycles []*LightCycle
)

func virtualRain() {
	solenoid.High()
	solenoidOpen = false
	for {
		if !solenoidForceClosed && !solenoidForceOpen {
			if !solenoidOpen {
				solenoid.Low()
				fmt.Println("Water valve open...")
				solenoidOpen = true
				time.Sleep(time.Second * 5)
			}
			if solenoidOpen {
				solenoid.High()
				fmt.Println("Water valve close...")
				solenoidOpen = false
				time.Sleep(time.Minute * 5)
			}
		}
	}
}

func virtualSun(cycles []*LightCycle) {
	//Set them all to off to start
	for _, cycle := range cycles {
		cycle.Pin().High()
		cycle.Active = false
	}

	//The main loop
	seconds := 0 //Only show time every 60 seconds
	for {
		hour, min, _ := time.Now().Clock()
		if seconds == 0 {
			fmt.Printf("%02d:%02d\n", hour, min)
			seconds++
		} else {
			seconds++
			if seconds == 60 {
				seconds = 0
			}
		}
		for _, cycle := range cycles {
			riseHour, riseMin := cycle.RiseClock()
			setHour, setMin := cycle.SetClock()
			if !cycle.ForceOn && !cycle.ForceOff { //We've not been overridden
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
		}
		time.Sleep(time.Second)
	}
}

func Status(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if solenoidForceOpen == true {
		fmt.Fprintf(w, "Water Mode: on\n")
	}
	if solenoidForceClosed == true {
		fmt.Fprintf(w, "Water Mode: off\n")
	}
	if solenoidForceOpen == false && solenoidForceClosed == false {
		fmt.Fprintf(w, "Water Mode: auto\n")
	}
	fmt.Fprintln(w, "========================")
	for _, cycle := range mainCycles {
		if cycle.ForceOn {
			fmt.Fprintf(w, "%s Mode: on\n", cycle.Name())
		}
		if cycle.ForceOff {
			fmt.Fprintf(w, "%s Mode: off\n", cycle.Name())
		}
		if cycle.ForceOn == false && cycle.ForceOff == false {
			fmt.Fprintf(w, "%s Mode: auto\n", cycle.Name())
		}
		fmt.Fprintln(w, "------------------------")
	}
}

func Lights(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	lightGroup := ps.ByName("lightGroup")
	setting := ps.ByName("setting")
	if setting == "on" || setting == "off" || setting == "auto" {
		cycleFound := false
		for _, cycle := range mainCycles {
			if cycle.Name() == lightGroup {
				if setting == "on" {
					cycle.ForceOn = true
					cycle.ForceOff = false
					cycle.Pin().Low()
					cycle.Active = true
				} else if setting == "off" {
					cycle.ForceOn = false
					cycle.ForceOff = true
					cycle.Pin().High()
					cycle.Active = false
				} else if setting == "auto" {
					cycle.ForceOff = false
					cycle.ForceOn = false
				}
				fmt.Fprintf(w, "%s Mode: %s\n", cycle.Name(), setting)
				fmt.Printf("User Override - %s Mode: %s\n", cycle.Name(), setting)
				cycleFound = true
				break
			}
		}
		if cycleFound == false {
			fmt.Fprintf(w, "Invalid cycle name.\n")
		}
	} else {
		fmt.Fprintf(w, "Invalid setting.\n")
	}
}

func Water(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	setting := ps.ByName("setting")
	setting = strings.ToLower(setting)
	if setting == "on" || setting == "off" || setting == "auto" {
		if setting == "on" {
			solenoidForceOpen = true
			solenoidForceClosed = false
			solenoid.Low()
			solenoidOpen = true
		} else if setting == "off" {
			solenoidForceOpen = false
			solenoidForceClosed = true
			solenoid.High()
			solenoidOpen = false
		} else if setting == "auto" {
			solenoidForceOpen = false
			solenoidForceClosed = false
		}
		fmt.Fprintf(w, "Water Mode: %s\n", setting)
		fmt.Printf("User Override - Water Mode: %s\n", setting)
	} else {
		fmt.Fprintf(w, "Invalid setting.\n")
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
	mainCycles = make([]*LightCycle, 1)
	mainCycles[0] = &LightCycle{"MainSun", lightGroup1, 6, 0, 22, 0, false, false, false} //Main sun rises at 6:00 and sets at 22:00

	//Run the system
	go virtualRain()
	go virtualSun(mainCycles)

	//HTTP
	router := httprouter.New()
	router.GET("/", Status)
	router.GET("/status", Status)
	router.GET("/lights/:lightGroup/:setting", Lights) //on, off, auto
	router.GET("/water/:setting", Water)               //on, off, auto
	log.Fatal(http.ListenAndServe(":80", router))
}
