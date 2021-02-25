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

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

const thermostatAPIURL = `https://api.ecobee.com/1/thermostat`
const thermostatSummaryURL = `https://api.ecobee.com/1/thermostatSummary`

func (c *Client) UpdateThermostat(utr UpdateThermostatRequest) error {
	j, err := json.Marshal(&utr)
	if err != nil {
		return fmt.Errorf("error marshaling json: %v", err)
	}

	glog.V(1).Infof("UpdateThermostat request: %s", j)

	// everything below here can be factored out into a common POST func
	resp, err := c.Post(thermostatAPIURL, "application/json", bytes.NewReader(j))
	if err != nil {
		return fmt.Errorf("error on post request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("invalid server response: %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading body: %v", err)
	}

	var s UpdateThermostatResponse
	if err = json.Unmarshal(body, &s); err != nil {
		return fmt.Errorf("error unmarshalling json: %v", err)
	}

	glog.V(1).Infof("UpdateThermostat response: %+v", s)

	if s.Status.Code == 0 {
		return nil
	}
	return fmt.Errorf("API error: %s", s.Status.Message)
}

func (c *Client) GetThermostat(thermostatID string) (*Thermostat, error) {
	// TODO: Consider factoring the generation of Selection out into
	// something else to make it more convenient to toggle the IncludeX
	// flags?
	s := Selection{
		SelectionType:  "thermostats",
		SelectionMatch: thermostatID,

		IncludeAlerts:          false,
		IncludeEvents:          true,
		IncludeProgram:         true,
		IncludeRuntime:         true,
		IncludeExtendedRuntime: true,
		IncludeSettings:        false,
		IncludeSensors:         true,
		IncludeWeather:         true,
	}
	thermostats, err := c.GetThermostats(s)
	if err != nil {
		return nil, err
	} else if len(thermostats) != 1 {
		return nil, fmt.Errorf("got %d thermostats, wanted 1", len(thermostats))
	}
	return &thermostats[0], nil
}

func (c *Client) GetThermostats(selection Selection) ([]Thermostat, error) {
	req := GetThermostatsRequest{
		Selection: selection,
	}
	j, err := json.Marshal(&req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json: %v", err)
	}

	body, err := c.get(thermostatAPIURL, j)
	if err != nil {
		return nil, fmt.Errorf("error fetching thermostats: %v", err)
	}

	var r GetThermostatsResponse
	if err = json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("error unmarshalling json: %v", err)
	}

	glog.V(1).Infof("GetThermostats response: %#v", r)

	if r.Status.Code != 0 {
		return nil, fmt.Errorf("api error %d: %v", r.Status.Code, r.Status.Message)
	}
	return r.ThermostatList, nil
}

func (c *Client) GetThermostatSummary(selection Selection) (map[string]ThermostatSummary, error) {
	req := GetThermostatSummaryRequest{
		Selection: selection,
	}
	j, err := json.Marshal(&req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json: %v", err)
	}

	body, err := c.get(thermostatSummaryURL, j)
	if err != nil {
		return nil, fmt.Errorf("error fetching thermostat summary: %v", err)
	}

	var r GetThermostatSummaryResponse
	if err = json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("error unmarshalling json: %v", err)
	}

	glog.V(1).Infof("GetThermostatSummary response: %#v", r)

	var tsm = make(ThermostatSummaryMap, r.ThermostatCount)

	for i := 0; i < r.ThermostatCount; i++ {
		rl := strings.Split(r.RevisionList[i], ":")
		if len(rl) < 7 {
			return nil, fmt.Errorf("invalid RevisionList, not enough fields: %s", r.RevisionList[i])
		}

		// Assume order of RevisionList and StatusList is the same.
		es, err := buildEquipmentStatus(r.StatusList[i])
		if err != nil {
			return nil, fmt.Errorf("error in buildEquipmentSTatus(%v): %v", r.StatusList[i], err)
		}

		connected, err := strconv.ParseBool(rl[2])
		if err != nil {
			return nil, fmt.Errorf("error from ParseBool(%v): %v", rl[2], err)
		}

		ts := ThermostatSummary{
			Identifier:         rl[0],
			Name:               rl[1],
			Connected:          connected,
			ThermostatRevision: rl[3],
			AlertsRevision:     rl[4],
			RuntimeRevision:    rl[5],
			IntervalRevision:   rl[6],
			EquipmentStatus:    es,
		}
		tsm[rl[0]] = ts
	}
	return tsm, nil
}

func (c *Client) get(endpoint string, rawRequest []byte) ([]byte, error) {

	glog.V(2).Infof("get(%s?json=%s)", endpoint, rawRequest)
	request := url.QueryEscape(string(rawRequest))
	resp, err := c.Get(fmt.Sprintf("%s?json=%s", endpoint, request))
	if err != nil {
		return nil, fmt.Errorf("error on get request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid server response: %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}

	glog.V(2).Infof("responses: %s", body)

	return body, nil
}

func buildEquipmentStatus(input string) (EquipmentStatus, error) {
	var es EquipmentStatus

	split := strings.SplitN(input, ":", 2)

	// Nothing on the right hand side.
	if len(split[1]) == 0 {
		return es, nil
	}

	statuses := strings.Split(split[1], ",")

	/* consider if this should be a switch statement instead of mucking with reflect */
	//v := reflect.ValueOf(&es).Elem()
	for _, s := range statuses {
		// f := v.FieldByName(strings.Title(s))
		// if f == reflect.Zero(v.Type()) {
		// 	glog.Infof("Unknown status %s from thermostat %s", s, id)
		// 	continue
		// }
		// f.SetBool(true)
		es.Set(s, true)
	}
	return es, nil
}

func (es *EquipmentStatus) Set(field string, state bool) {

	switch field {
	case "heatPump":
		es.HeatPump = state
	case "heatPump2":
		es.HeatPump2 = state
	case "heatPump3":
		es.HeatPump3 = state
	case "compCool1":
		es.CompCool1 = state
	case "compCool2":
		es.CompCool2 = state
	case "auxHeat1":
		es.AuxHeat1 = state
	case "auxHeat2":
		es.AuxHeat2 = state
	case "auxHeat3":
		es.AuxHeat3 = state
	case "fan":
		es.Fan = state
	case "humidifier":
		es.Humidifier = state
	case "dehumidifier":
		es.Dehumidifier = state
	case "ventilator":
		es.Ventilator = state
	case "economizer":
		es.Economizer = state
	case "compHotWater":
		es.CompHotWater = state
	case "auxHotWater":
		es.AuxHotWater = state
	}

}
