package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/avast/retry-go"
	"github.com/cdzombak/libwx"
	"github.com/influxdata/influxdb-client-go/v2"

	"ecobee_influx_connector/ecobee" // taken from https://github.com/rspier/go-ecobee and lightly customized
)

type Config struct {
	APIKey                    string `json:"api_key"`
	WorkDir                   string `json:"work_dir,omitempty"`
	ThermostatID              string `json:"thermostat_id"`
	InfluxServer              string `json:"influx_server"`
	InfluxOrg                 string `json:"influx_org,omitempty"`
	InfluxUser                string `json:"influx_user,omitempty"`
	InfluxPass                string `json:"influx_password,omitempty"`
	InfluxToken               string `json:"influx_token,omitempty"`
	InfluxBucket              string `json:"influx_bucket"`
	InfluxHealthCheckDisabled bool   `json:"influx_health_check_disabled"`
	WriteHeatPump1            bool   `json:"write_heat_pump_1"`
	WriteHeatPump2            bool   `json:"write_heat_pump_2"`
	WriteAuxHeat1             bool   `json:"write_aux_heat_1"`
	WriteAuxHeat2             bool   `json:"write_aux_heat_2"`
	WriteCool1                bool   `json:"write_cool_1"`
	WriteCool2                bool   `json:"write_cool_2"`
	WriteHumidifier           bool   `json:"write_humidifier"`
	WriteDehumidifier         bool   `json:"write_dehumidifier"`
	AlwaysWriteWeather        bool   `json:"always_write_weather_as_current"`
	TemperatureUnits          string `json:"temperature_units"`
}

const (
	thermostatNameTag            = "thermostat_name"
	source                       = "ecobee"
	sourceTag                    = "data_source"
	ecobeeWeatherMeasurementName = "ecobee_weather"
)

func main() {
	var configFile = flag.String("config", "", "Configuration JSON file.")
	var listThermostats = flag.Bool("list-thermostats", false, "List available thermostats, then exit.")
	flag.Parse()

	if *configFile == "" {
		fmt.Println("-config is required.")
		os.Exit(1)
	}

	config := Config{}
	cfgBytes, err := os.ReadFile(*configFile)
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

	const influxTimeout = 3 * time.Second
	authString := ""
	if config.InfluxUser != "" || config.InfluxPass != "" {
		authString = fmt.Sprintf("%s:%s", config.InfluxUser, config.InfluxPass)
	} else if config.InfluxToken != "" {
		authString = fmt.Sprintf("%s", config.InfluxToken)
	}
	influxClient := influxdb2.NewClient(config.InfluxServer, authString)
	if !config.InfluxHealthCheckDisabled {
		ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
		defer cancel()
		health, err := influxClient.Health(ctx)
		if err != nil {
			log.Fatalf("failed to check InfluxDB health: %v", err)
		}
		if health.Status != "pass" {
			log.Fatalf("InfluxDB did not pass health check: status %s; message '%s'", health.Status, *health.Message)
		}
	}
	influxWriteApi := influxClient.WriteAPIBlocking(config.InfluxOrg, config.InfluxBucket)
	_ = influxWriteApi

	lastWrittenRuntimeInterval := 0
	lastWrittenWeather := time.Time{}
	lastWrittenSensors := time.Time{}

	// Converts default Fahrenheit to Celsius based on config.json
	temperatureConverter := func(temp float64, units string) float64 {
		if units == "C" {
			return (temp - 32) * 5 / 9 // Fahrenheit to Celsius conversion
		}
		return temp // Default to Fahrenheit
	}

	doUpdate := func() {
		if err := retry.Do(
			func() error {
				t, err := client.GetThermostat(config.ThermostatID)
				if err != nil {
					return err
				}

				// Air quality related values are only in the current runtime,
				// thus they need to be handled outside of the extended runtime section
				currentRuntimeReportTime, err := time.Parse("2006-01-02 15:04:05", t.Runtime.LastStatusModified)
				if err != nil {
					return err
				}

				if err := retry.Do(func() error {
					ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
					defer cancel()

					actualAQAccuracy := t.Runtime.ActualAQAccuracy
					actualAQScore := t.Runtime.ActualAQScore
					actualCO2 := t.Runtime.ActualCO2
					actualVOC := t.Runtime.ActualVOC

					fmt.Printf("Air quality at %s:\n", currentRuntimeReportTime)
					fmt.Printf("\tcurrent co2: %d\n\tcurrent voc: %d\n",
						actualCO2, actualVOC)

					fields := map[string]interface{}{
						"airquality_accuracy": actualAQAccuracy,
						"airquality_score":    actualAQScore,
						"co2":                 actualCO2,
						"voc":                 actualVOC,
					}

					err := influxWriteApi.WritePoint(ctx,
						influxdb2.NewPoint(
							"ecobee_air_quality",
							map[string]string{thermostatNameTag: t.Name}, // tags
							fields,
							currentRuntimeReportTime,
						))
					if err != nil {
						return err
					}
					return nil
				}, retry.Attempts(3), retry.Delay(1*time.Second)); err != nil {
					return err
				}

				latestRuntimeInterval := t.ExtendedRuntime.RuntimeInterval
				log.Printf("latest runtime interval available is %d\n", latestRuntimeInterval)

				// In the absence of a time zone indicator, Parse returns a time in UTC.
				baseReportTime, err := time.Parse("2006-01-02 15:04:05", t.ExtendedRuntime.LastReadingTimestamp)
				if err != nil {
					return err
				}

				for i := 0; i < 3; i++ {
					reportTime := baseReportTime
					if i == 0 {
						reportTime = reportTime.Add(-5 * time.Minute)
					}
					if i == 2 {
						reportTime = reportTime.Add(5 * time.Minute)
					}

					currentTemp := temperatureConverter(float64(t.ExtendedRuntime.ActualTemperature[i])/10.0, config.TemperatureUnits)
					currentHumidity := t.ExtendedRuntime.ActualHumidity[i]
					heatSetPoint := temperatureConverter(float64(t.ExtendedRuntime.DesiredHeat[i])/10.0, config.TemperatureUnits)
					coolSetPoint := temperatureConverter(float64(t.ExtendedRuntime.DesiredCool[i])/10.0, config.TemperatureUnits)
					humiditySetPoint := t.ExtendedRuntime.DesiredHumidity[i]
					demandMgmtOffset := float64(t.ExtendedRuntime.DmOffset[i]) / 10.0
					hvacMode := t.ExtendedRuntime.HvacMode[i] // string :(
					heatPump1RunSec := t.ExtendedRuntime.HeatPump1[i]
					heatPump2RunSec := t.ExtendedRuntime.HeatPump2[i]
					auxHeat1RunSec := t.ExtendedRuntime.AuxHeat1[i]
					auxHeat2RunSec := t.ExtendedRuntime.AuxHeat2[i]
					cool1RunSec := t.ExtendedRuntime.Cool1[i]
					cool2RunSec := t.ExtendedRuntime.Cool2[i]
					fanRunSec := t.ExtendedRuntime.Fan[i]
					humidifierRunSec := t.ExtendedRuntime.Humidifier[i]
					dehumidifierRunSec := t.ExtendedRuntime.Dehumidifier[i]

					if config.TemperatureUnits == "C" {
						fmt.Printf("Thermostat conditions at %s:\n", reportTime)
						fmt.Printf("\tcurrent temperature: %.1f degC\n\theat set point: %.1f degC\n\tcool set point: %.1f degC\n\tdemand management offset: %.1f\n",
							currentTemp, heatSetPoint, coolSetPoint, demandMgmtOffset)
						fmt.Printf("\tcurrent humidity: %d%%\n\thumidity set point: %d\n\tHVAC mode: %s\n",
							currentHumidity, humiditySetPoint, hvacMode)
						fmt.Printf("\tfan runtime: %d seconds\n\thumidifier runtime: %d seconds\n\tdehumidifier runtime: %d seconds\n",
							fanRunSec, humidifierRunSec, dehumidifierRunSec)
						fmt.Printf("\theat pump 1 runtime: %d seconds\n\theat pump 2 runtime: %d seconds\n",
							heatPump1RunSec, heatPump2RunSec)
						fmt.Printf("\theat 1 runtime: %d seconds\n\theat 2 runtime: %d seconds\n",
							auxHeat1RunSec, auxHeat2RunSec)
						fmt.Printf("\tcool 1 runtime: %d seconds\n\tcool 2 runtime: %d seconds\n",
							cool1RunSec, cool2RunSec)
					} else {
						fmt.Printf("Thermostat conditions at %s:\n", reportTime)
						fmt.Printf("\tcurrent temperature: %.1f degF\n\theat set point: %.1f degF\n\tcool set point: %.1f degF\n\tdemand management offset: %.1f\n",
							currentTemp, heatSetPoint, coolSetPoint, demandMgmtOffset)
						fmt.Printf("\tcurrent humidity: %d%%\n\thumidity set point: %d\n\tHVAC mode: %s\n",
							currentHumidity, humiditySetPoint, hvacMode)
						fmt.Printf("\tfan runtime: %d seconds\n\thumidifier runtime: %d seconds\n\tdehumidifier runtime: %d seconds\n",
							fanRunSec, humidifierRunSec, dehumidifierRunSec)
						fmt.Printf("\theat pump 1 runtime: %d seconds\n\theat pump 2 runtime: %d seconds\n",
							heatPump1RunSec, heatPump2RunSec)
						fmt.Printf("\theat 1 runtime: %d seconds\n\theat 2 runtime: %d seconds\n",
							auxHeat1RunSec, auxHeat2RunSec)
						fmt.Printf("\tcool 1 runtime: %d seconds\n\tcool 2 runtime: %d seconds\n",
							cool1RunSec, cool2RunSec)
					}

					if latestRuntimeInterval != lastWrittenRuntimeInterval {
						if err := retry.Do(func() error {
							ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
							defer cancel()
							fields := map[string]interface{}{
								"temperature":        temperatureConverter(currentTemp, "C"),
								"humidity":           currentHumidity,
								"heat_set_point":     temperatureConverter(heatSetPoint, "C"),
								"cool_set_point":     temperatureConverter(coolSetPoint, "C"),
								"demand_mgmt_offset": demandMgmtOffset,
								"fan_run_time":       fanRunSec,
							}
							if config.WriteHumidifier || config.WriteDehumidifier {
								fields["humidity_set_point"] = humiditySetPoint
							}

							if config.WriteHumidifier {
								fields["humidifier_run_time"] = humidifierRunSec
							}

							if config.WriteDehumidifier {
								fields["dehumidifier_run_time"] = dehumidifierRunSec
							}
							if config.WriteAuxHeat1 {
								fields["aux_heat_1_run_time"] = auxHeat1RunSec
							}
							if config.WriteAuxHeat2 {
								fields["aux_heat_2_run_time"] = auxHeat2RunSec
							}
							if config.WriteHeatPump1 {
								fields["heat_pump_1_run_time"] = heatPump1RunSec
							}
							if config.WriteHeatPump2 {
								fields["heat_pump_2_run_time"] = heatPump2RunSec
							}
							if config.WriteCool1 {
								fields["cool_1_run_time"] = cool1RunSec
							}
							if config.WriteCool2 {
								fields["cool_2_run_time"] = cool2RunSec
							}
							err := influxWriteApi.WritePoint(ctx,
								influxdb2.NewPoint(
									"ecobee_runtime",
									map[string]string{thermostatNameTag: t.Name}, // tags
									fields,
									reportTime,
								))
							if err != nil {
								return err
							}
							return nil
						}, retry.Attempts(3), retry.Delay(1*time.Second)); err != nil {
							return err
						}
					}
				}
				lastWrittenRuntimeInterval = latestRuntimeInterval

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
								temp = temperatureConverter(float64(tempInt)/10.0, config.TemperatureUnits)
							}
						} else if c.Type == "occupancy" {
							presenceSupported = true
							presence = c.Value == "true"
						}
					}

					fmt.Printf("Sensor '%s' at %s:\n", name, sensorTime)

					if config.TemperatureUnits == "C" {
						fmt.Printf("\ttemperature: %.1f degC\n", temp)
					} else {
						fmt.Printf("\ttemperature: %.1f degF\n", temp)
					}

					if presenceSupported {
						fmt.Printf("\toccupied: %t\n", presence)
					}

					if temp == 0.0 {
						continue
					}

					if sensorTime != lastWrittenSensors {
						if err := retry.Do(func() error {
							ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
							defer cancel()
							fields := map[string]interface{}{
								"temperature": temp,
							}
							if presenceSupported {
								fields["occupied"] = presence
							}
							err := influxWriteApi.WritePoint(ctx,
								influxdb2.NewPoint(
									"ecobee_sensor",
									map[string]string{
										thermostatNameTag: t.Name,
										"sensor_name":     sensor.Name,
										"sensor_id":       sensor.ID,
									}, // tags
									fields,
									sensorTime,
								))
							if err != nil {
								return err
							}
							return nil
						}, retry.Attempts(3), retry.Delay(1*time.Second)); err != nil {
							return err
						}
					}
				}
				lastWrittenSensors = sensorTime

				weatherTime, err := time.Parse("2006-01-02 15:04:05", t.Weather.Timestamp)
				if err != nil {
					return err
				}
				outdoorTemp := temperatureConverter(float64(t.Weather.Forecasts[0].Temperature)/10.0, config.TemperatureUnits)
				pressureMillibar := t.Weather.Forecasts[0].Pressure
				outdoorHumidity := t.Weather.Forecasts[0].RelativeHumidity
				dewpoint := temperatureConverter(float64(t.Weather.Forecasts[0].Dewpoint)/10.0, config.TemperatureUnits)
				windSpeedMph := t.Weather.Forecasts[0].WindSpeed
				windBearing := t.Weather.Forecasts[0].WindBearing
				visibilityMeters := t.Weather.Forecasts[0].Visibility
				visibilityMiles := float64(visibilityMeters) / 1609.34
				windChill := libwx.WindChillF(libwx.TempF(outdoorTemp), float64(windSpeedMph))
				weatherSymbol := t.Weather.Forecasts[0].WeatherSymbol
				sky := t.Weather.Forecasts[0].Sky

				fmt.Printf("Weather at %s:\n", weatherTime)

				if config.TemperatureUnits == "C" {
					fmt.Printf("\ttemperature: %.1f degC\n\tpressure: %d mb\n\thumidity: %d%%\n\tdew point: %.1f degC\n\twind: %d at %d mph\n\twind chill: %.1f degC\n\tvisibility: %.1f miles\nweather symbol: %d\nsky: %d",
						outdoorTemp, pressureMillibar, outdoorHumidity, dewpoint, windBearing, windSpeedMph, windChill, visibilityMiles, weatherSymbol, sky)
				} else {
					fmt.Printf("\ttemperature: %.1f degF\n\tpressure: %d mb\n\thumidity: %d%%\n\tdew point: %.1f degF\n\twind: %d at %d mph\n\twind chill: %.1f degF\n\tvisibility: %.1f miles\nweather symbol: %d\nsky: %d",
						outdoorTemp, pressureMillibar, outdoorHumidity, dewpoint, windBearing, windSpeedMph, windChill, visibilityMiles, weatherSymbol, sky)
				}

				if weatherTime != lastWrittenWeather || config.AlwaysWriteWeather {
					if err := retry.Do(func() error {
						ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
						defer cancel()
						pointTime := weatherTime
						if config.AlwaysWriteWeather {
							pointTime = time.Now()
						}
						err := influxWriteApi.WritePoint(ctx,
							influxdb2.NewPoint(
								ecobeeWeatherMeasurementName,
								map[string]string{ // tags
									thermostatNameTag: t.Name,
									sourceTag:         source,
								},
								map[string]interface{}{ // fields
									"outdoor_temp":                    temperatureConverter(outdoorTemp, "C"),
									"outdoor_humidity":                outdoorHumidity,
									"barometric_pressure_mb":          pressureMillibar,
									"barometric_pressure_inHg":        float64(pressureMillibar) / 33.864,
									"dew_point":                       temperatureConverter(dewpoint, "C"),
									"wind_speed":                      windSpeedMph,
									"wind_bearing":                    windBearing,
									"visibility_mi":                   visibilityMiles,
									"recommended_max_indoor_humidity": libwx.IndoorHumidityRecommendationF(libwx.TempF(outdoorTemp)),
									"wind_chill_f":                    windChill,
									"weather_symbol":                  weatherSymbol,
									"sky":                             sky,
								},
								pointTime,
							))
						if err != nil {
							return err
						}
						lastWrittenWeather = weatherTime
						return nil
					}, retry.Attempts(3), retry.Delay(1*time.Second)); err != nil {
						return err
					}
				}

				return nil
			},
			retry.Attempts(3),
			retry.Delay(5*time.Second),
		); err != nil {
			log.Fatal(err)
		}
	}

	doUpdate()
	for {
		select {
		case <-time.Tick(5 * time.Minute):
			doUpdate()
		}
	}
}
