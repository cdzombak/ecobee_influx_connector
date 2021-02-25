package ecobee

// Copyright 2017 Google Inc.
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

// The functions in this file aren't part of the official API, but are
// useful helpers.

import (
	"fmt"
	"time"
)

func (c *Client) ResumeProgram(id string, resumeAll bool) error {
	r := &UpdateThermostatRequest{
		Selection: Selection{
			SelectionType:  "thermostats",
			SelectionMatch: id,
		},
		Functions: []Function{
			Function{
				Type: "resumeProgram",
				Params: ResumeProgramParams{
					ResumeAll: resumeAll,
				},
			},
		},
	}
	return c.UpdateThermostat(*r)
}

func (c *Client) RunFan(id string, duration time.Duration) error {
	end := time.Now().Add(duration)
	shp := SetHoldParams{
		// these HoldTemps don't get used because the IsTemperature
		// flags are both false.
		CoolHoldTemp: 800,
		HeatHoldTemp: 690,
		HoldType:     "dateTime",
		EndTime:      end.Format("15:04:05"),
		EndDate:      end.Format("2006-01-02"),
		Event: Event{
			Fan:                   "on",
			IsTemperatureRelative: false,
			IsTemperatureAbsolute: false,
			OccupiedSensorActive:  false,
		},
	}
	r := &UpdateThermostatRequest{
		Selection: Selection{
			SelectionType:  "thermostats",
			SelectionMatch: id,
		},
		Functions: []Function{
			Function{
				Type:   "setHold",
				Params: shp,
			},
		},
	}

	return c.UpdateThermostat(*r)
}

func (c *Client) SendMessage(thermostat, message string) error {
	smp := SendMessageParams{
		Alert: Alert{
			AlertType:       "message",
			IsOperatorAlert: true,
		},
		Text: message,
	}

	r := &UpdateThermostatRequest{
		Selection: Selection{
			SelectionType:  "thermostats",
			SelectionMatch: thermostat,
		},
		Functions: []Function{
			Function{
				Type:   "sendMessage",
				Params: smp,
			},
		},
	}

	return c.UpdateThermostat(*r)
}

// The Ecobee API represents temperatures as integers.
func makeTemp(h, c float64) (int, int) {
	return int(h * 10), int(c * 10)
}

func tempCheck(heat, cool float64) error {
	// Note: two properties Runtime.desiredCoolRange and
	// Runtime.desiredHeatRange indicate the current valid temperature
	// ranges. These fields should be queried before using the SetHold
	// function in order to mitigate against the desired setpointsld
	// being adjusted by the server when the values are not within the
	// valid ranges.
	//DesiredHeatRange:[]int{450, 860}, DesiredCoolRange:[]int{600, 920}}}

	if heat == 0 {
		return fmt.Errorf("heat must not be 0")
	}
	if cool == 0 {
		return fmt.Errorf("cool must not be 0")
	}
	if heat > 90 {
		return fmt.Errorf("heat %.1f above limit %d", heat, 90)
	}
	if cool < 60 {
		return fmt.Errorf("cool %.1f below limit %d", cool, 60)
	}
	if cool < heat {
		return fmt.Errorf("heat %.1f must be below cool %.1f", heat, cool)
	}
	return nil
}

func (c *Client) HoldTemp(thermostat string, heat, cool float64, d time.Duration) error {
	end := time.Now().Add(d)

	if err := tempCheck(heat, cool); err != nil {
		return err
	}

	ht, cl := makeTemp(heat, cool)

	shp := SetHoldParams{
		HeatHoldTemp: ht,
		CoolHoldTemp: cl,

		HoldType: "dateTime",
		EndTime:  end.Format("15:04:05"),
		EndDate:  end.Format("2006-01-02"),
		Event: Event{
			Fan: "auto",
			// relative temperatures don't seem to work with the API.
			IsTemperatureRelative: false,
			IsTemperatureAbsolute: true,
			OccupiedSensorActive:  false,
		},
	}

	r := &UpdateThermostatRequest{
		Selection: Selection{
			SelectionType:  "thermostats",
			SelectionMatch: thermostat,
		},
		Functions: []Function{
			Function{
				Type:   "setHold",
				Params: shp,
			},
		},
	}

	return c.UpdateThermostat(*r)
}
