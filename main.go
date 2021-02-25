package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/avast/retry-go"

	"ecobee_influx_connector/ecobee" // taken from https://github.com/rspier/go-ecobee and lightly customized
)

type Config struct {
	APIKey          string `json:"api_key"`
	WorkDir         string `json:"work_dir,omitempty"`
	ThermostatID    string `json:"thermostat_id"`
	InfluxServer    string `json:"influx_server"`
	InfluxUser      string `json:"influx_user,omitempty"`
	InfluxPass      string `json:"influx_password,omitempty"`
	InfluxBucket    string `json:"influx_bucket"`
	WriteHeatPump1  bool   `json:"write_heat_pump_1"`
	WriteHeatPump2  bool   `json:"write_heat_pump_2"`
	WriteAuxHeat1   bool   `json:"write_aux_heat_1"`
	WriteAuxHeat2   bool   `json:"write_aux_heat_2"`
	WriteCool1      bool   `json:"write_cool_1"`
	WriteCool2      bool   `json:"write_cool_2"`
	WriteHumidifier bool   `json:"write_humidifier"`
}

func main() {
	var configFile = flag.String("config", "", "Configuration JSON file.")
	var listThermostats = flag.Bool("list-thermostats", false, "List available thermostats, then exit.")
	flag.Parse()

	if *configFile == "" {
		fmt.Println("-config is required.")
		os.Exit(1)
	}

	config := Config{}
	cfgBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("Unable to read config file '%s': %s", *configFile, err)
	}
	if err = json.Unmarshal(cfgBytes, &config); err != nil {
		log.Fatalf("Unable to parse config file '%s': %s", *configFile, err)
	}
	if config.APIKey == "" {
		log.Fatal("api_key must be set in the config file.")
	}
	if config.WorkDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Unable to get current working directory: %s", err)
		}
		config.WorkDir = wd
	}

	client := ecobee.NewClient(config.APIKey, path.Join(config.WorkDir, "ecobee-cred-cache"))

	if *listThermostats {
		s := ecobee.Selection{
			SelectionType: "registered",
		}
		ts, err := client.GetThermostats(s)
		if err != nil {
			log.Fatal(err)
		}
		for _, t := range ts {
			fmt.Printf("'%s': ID %s\n", t.Name, t.Identifier)
		}
		os.Exit(0)
	}

	if config.ThermostatID == "" {
		log.Fatalf("thermostat_id must be set in the config file.")
	}
	if config.InfluxBucket == "" || config.InfluxServer == "" {
		log.Fatalf("influx_server and influx_bucket must be set in the config file.")
	}

	// TODO(cdzombak): do every 10 min
	// TODO(cdzombak): actually write to Influx

	if err := retry.Do(
		func() error {
			t, err := client.GetThermostat(config.ThermostatID)
			if err != nil {
				return err
			}

			latestRuntimeInterval := t.ExtendedRuntime.RuntimeInterval
			log.Printf("latest runtime interval available is %d\n", latestRuntimeInterval)

			baseReportTime, err := time.Parse("2006-01-02 15:04:05", t.ExtendedRuntime.LastReadingTimestamp)
			if err != nil {
				return err
			}

			for i := 0; i < 3; i++ {
				// In the absence of a time zone indicator, Parse returns a time in UTC.
				reportTime := baseReportTime.Add(time.Duration(-5*i) * time.Minute)

				currentTemp := float64(t.ExtendedRuntime.ActualTemperature[i]) / 10.0
				currentHumidity := t.ExtendedRuntime.ActualHumidity[i]
				heatSetPoint := float64(t.ExtendedRuntime.DesiredHeat[i]) / 10.0
				coolSetPoint := float64(t.ExtendedRuntime.DesiredCool[i]) / 10.0
				humiditySetPoint := t.ExtendedRuntime.DesiredHumidity[i]
				demandMgmtOffset := float64(t.ExtendedRuntime.DmOffset[i]) / 10.0
				hvacMode := t.ExtendedRuntime.HvacMode[i] // string :(
				heatPump1RunSec := t.ExtendedRuntime.HeatPump1[i]
				heatPump2RunSec := t.ExtendedRuntime.HeatPump1[i]
				auxHeat1RunSec := t.ExtendedRuntime.AuxHeat1[i]
				auxHeat2RunSec := t.ExtendedRuntime.AuxHeat2[i]
				cool1RunSec := t.ExtendedRuntime.Cool1[i]
				cool2RunSec := t.ExtendedRuntime.Cool2[i]
				fanRunSec := t.ExtendedRuntime.Fan[i]
				humidifierRunSec := t.ExtendedRuntime.Humidifier[i]

				fmt.Printf("Thermostat conditions at %s:\n", reportTime)
				fmt.Printf("\tcurrent temperature: %.1f degF\n\theat set point: %.1f degF\n\tcool set point: %.1f degF\n\tdemand management offset: %.1f\n",
					currentTemp, heatSetPoint, coolSetPoint, demandMgmtOffset)
				fmt.Printf("\tcurrent humidity: %d%%\n\thumidity set point: %d\n\tHVAC mode: %s\n",
					currentHumidity, humiditySetPoint, hvacMode)
				fmt.Printf("\tfan runtime: %d seconds\n\thumidifier runtime: %d seconds\n",
					fanRunSec, humidifierRunSec)
				fmt.Printf("\theat pump 1 runtime: %d seconds\n\theat pump 2 runtime: %d seconds\n",
					heatPump1RunSec, heatPump2RunSec)
				fmt.Printf("\theat 1 runtime: %d seconds\n\theat 2 runtime: %d seconds\n",
					auxHeat1RunSec, auxHeat2RunSec)
				fmt.Printf("\tcool 1 runtime: %d seconds\n\tcool 2 runtime: %d seconds\n",
					cool1RunSec, cool2RunSec)
			}

			// assume t.LastModified for these:
			sensorTime, err := time.Parse("2006-01-02 15:04:05", t.UtcTime)
			if err != nil {
				return err
			}
			for _, sensor := range t.RemoteSensors {
				name := sensor.Name
				var temp float64
				var presence, presenceSupported bool
				for _, c := range sensor.Capability {
					if c.Type == "temperature" {
						tempInt, err := strconv.Atoi(c.Value)
						if err != nil {
							log.Printf("error reading temp '%s' for sensor %s: %s", c.Value, sensor.Name, err)
						} else {
							temp = float64(tempInt) / 10.0
						}
					} else if c.Type == "occupancy" {
						presenceSupported = true
						presence = c.Value == "true"
					}
				}
				fmt.Printf("Sensor '%s' at %s:\n", name, sensorTime)
				fmt.Printf("\ttemperature: %.1f degF\n", temp)
				if presenceSupported {
					fmt.Printf("\toccupied: %t\n", presence)
				}
			}

			weatherTime, err := time.Parse("2006-01-02 15:04:05", t.Weather.Timestamp)
			if err != nil {
				return err
			}
			outdoorTemp := float64(t.Weather.Forecasts[0].Temperature) / 10.0
			pressureMillibar := t.Weather.Forecasts[0].Pressure
			outdoorHumidity := t.Weather.Forecasts[0].RelativeHumidity
			dewpoint := float64(t.Weather.Forecasts[0].Dewpoint) / 10.0
			windspeedMph := t.Weather.Forecasts[0].WindSpeed

			fmt.Printf("Weather at %s:\n", weatherTime)
			fmt.Printf("\ttemperature: %.1f degF\n\tpressure: %d mb\n\thumidity: %d%%\n\tdew point: %.1f degF\n\twind speed: %d mph\n",
				outdoorTemp, pressureMillibar, outdoorHumidity, dewpoint, windspeedMph)

			return nil
		},
	); err != nil {
		log.Fatal(err)
	}
}
