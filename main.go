package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/avast/retry-go"
	wx "github.com/cdzombak/libwx"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/influxdata/influxdb-client-go/v2"
	influxdb2api "github.com/influxdata/influxdb-client-go/v2/api"

	"ecobee_influx_connector/ecobee" // taken from https://github.com/rspier/go-ecobee and lightly customized
)

type MQTTConfig struct {
	Enabled   bool   `json:"enabled"`
	Server    string `json:"server"`
	Port      int    `json:"port,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	TopicRoot string `json:"topic_root"`
}

// Config describes the ecobee_influx_connector program's configuration.
// It is used to parse the configuration JSON file.
type Config struct {
	APIKey                    string     `json:"api_key"`
	WorkDir                   string     `json:"work_dir,omitempty"`
	ThermostatID              string     `json:"thermostat_id"`
	InfluxServer              string     `json:"influx_server"`
	InfluxOrg                 string     `json:"influx_org,omitempty"`
	InfluxUser                string     `json:"influx_user,omitempty"`
	InfluxPass                string     `json:"influx_password,omitempty"`
	InfluxToken               string     `json:"influx_token,omitempty"`
	InfluxBucket              string     `json:"influx_bucket"`
	InfluxHealthCheckDisabled bool       `json:"influx_health_check_disabled"`
	MQTT                      MQTTConfig `json:"mqtt"`
	WriteHeatPump1            bool       `json:"write_heat_pump_1"`
	WriteHeatPump2            bool       `json:"write_heat_pump_2"`
	WriteAuxHeat1             bool       `json:"write_aux_heat_1"`
	WriteAuxHeat2             bool       `json:"write_aux_heat_2"`
	WriteCool1                bool       `json:"write_cool_1"`
	WriteCool2                bool       `json:"write_cool_2"`
	WriteHumidifier           bool       `json:"write_humidifier"`
	WriteDehumidifier         bool       `json:"write_dehumidifier"`
	AlwaysWriteWeather        bool       `json:"always_write_weather_as_current"`
}

// TODO(cdzombak): config v2:
// - separate write config and influx config into their own sections
// - add boolean for influx enabled

const (
	thermostatNameTag            = "thermostat_name"
	source                       = "ecobee"
	sourceTag                    = "data_source"
	ecobeeWeatherMeasurementName = "ecobee_weather"
)

// publishToMQTT publishes a value to an MQTT topic
// TODO(cdzombak): return an error, but this is going to be messy and need refactoring
func publishToMQTT(client mqtt.Client, topicRoot string, topicPath string, value interface{}) {
	if client == nil {
		return
	}

	topic := fmt.Sprintf("%s/%s", topicRoot, topicPath)
	payload := fmt.Sprintf("%v", value)

	token := client.Publish(topic, 0, false, payload)
	// TODO(cdzombak): make timeout configurable
	if !token.WaitTimeout(3 * time.Second) {
		log.Printf("Timeout publishing to MQTT topic %s", topic)
		return
	}
	if token.Error() != nil {
		log.Printf("Error publishing to MQTT topic %s: %v", topic, token.Error())
	}
}

var version = "<dev>"

func main() {
	configFile := flag.String("config", "", "Configuration JSON file.")
	listThermostats := flag.Bool("list-thermostats", false, "List available thermostats, then exit.")
	printVersion := flag.Bool("version", false, "Print version and exit.")
	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		os.Exit(0)
	}

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

	var influxClient influxdb2.Client
	var influxWriteAPI influxdb2api.WriteAPIBlocking
	influxEnabled := config.InfluxServer != "" && config.InfluxBucket != ""
	// TODO(cdzombak): make timeout configurable
	const influxTimeout = 3 * time.Second

	if influxEnabled {
		authString := ""
		if config.InfluxUser != "" || config.InfluxPass != "" {
			authString = fmt.Sprintf("%s:%s", config.InfluxUser, config.InfluxPass)
		} else if config.InfluxToken != "" {
			authString = fmt.Sprintf("%s", config.InfluxToken)
		}
		influxClient = influxdb2.NewClient(config.InfluxServer, authString)
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
		influxWriteAPI = influxClient.WriteAPIBlocking(config.InfluxOrg, config.InfluxBucket)
		log.Printf("Connected to InfluxDB at %s", config.InfluxServer)
	} else {
		log.Printf("InfluxDB is not configured, data will not be written to InfluxDB")
	}

	var mqttClient mqtt.Client
	mqttEnabled := config.MQTT.Enabled
	if mqttEnabled {
		if config.MQTT.Server == "" || config.MQTT.TopicRoot == "" {
			log.Fatalf("MQTT is enabled but server or topic_root is not set in the config file.")
		}

		opts := mqtt.NewClientOptions()
		port := config.MQTT.Port
		if port == 0 {
			port = 1883 // Default MQTT port
		}
		broker := fmt.Sprintf("tcp://%s:%d", config.MQTT.Server, port)
		opts.AddBroker(broker)

		if config.MQTT.Username != "" {
			opts.SetUsername(config.MQTT.Username)
			opts.SetPassword(config.MQTT.Password)
		}

		opts.SetClientID(fmt.Sprintf("ecobee_influx_connector_%d", time.Now().Unix()))
		opts.SetAutoReconnect(true)
		opts.SetConnectRetry(true)

		mqttClient = mqtt.NewClient(opts)
		if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.Fatalf("Failed to connect to MQTT broker: %v", token.Error())
		}

		log.Printf("Connected to MQTT broker at %s", broker)
	}

	// Require at least one output method to be enabled:
	if !influxEnabled && !mqttEnabled {
		log.Fatalf("At least one output method (InfluxDB or MQTT) must be configured")
	}

	lastWrittenRuntimeInterval := 0
	lastWrittenWeather := time.Time{}
	lastWrittenSensors := time.Time{}

	doUpdate := func() {
		if err := retry.Do(
			func() error {
				t, err := client.GetThermostat(config.ThermostatID)
				if err != nil {
					return err
				}

				// Air quality related values are only in the current runtime,
				// thus they need to be handled outside the extended runtime section
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

					if influxEnabled {
						err := influxWriteAPI.WritePoint(ctx,
							influxdb2.NewPoint(
								"ecobee_air_quality",
								map[string]string{thermostatNameTag: t.Name}, // tags
								fields,
								currentRuntimeReportTime,
							))
						if err != nil {
							return err
						}
					}

					// Publish to MQTT if enabled
					if config.MQTT.Enabled && mqttClient != nil {
						publishToMQTT(mqttClient, config.MQTT.TopicRoot, "sensor/airquality_accuracy", actualAQAccuracy)
						publishToMQTT(mqttClient, config.MQTT.TopicRoot, "sensor/airquality_score", actualAQScore)
						publishToMQTT(mqttClient, config.MQTT.TopicRoot, "sensor/co2", actualCO2)
						publishToMQTT(mqttClient, config.MQTT.TopicRoot, "sensor/voc", actualVOC)
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

					currentTemp := wx.TempF(float64(t.ExtendedRuntime.ActualTemperature[i]) / 10.0)
					currentHumidity := t.ExtendedRuntime.ActualHumidity[i]
					heatSetPoint := wx.TempF(float64(t.ExtendedRuntime.DesiredHeat[i]) / 10.0)
					coolSetPoint := wx.TempF(float64(t.ExtendedRuntime.DesiredCool[i]) / 10.0)
					humiditySetPoint := t.ExtendedRuntime.DesiredHumidity[i]
					demandMgmtOffset := wx.TempF(float64(t.ExtendedRuntime.DmOffset[i]) / 10.0)
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

					fmt.Printf("Thermostat conditions at %s:\n", reportTime)
					fmt.Printf("\tcurrent temperature: %.1f degF (%.1f degC)\n\theat set point: %.1f degF (%.1f degC)"+
						"\n\tcool set point: %.1f degF (%.1f degC)\n\tdemand management offset: %.1f (%.1f degC)\n",
						currentTemp, currentTemp.C(), heatSetPoint, heatSetPoint.C(),
						coolSetPoint, coolSetPoint.C(), demandMgmtOffset, demandMgmtOffset.C())
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

					if latestRuntimeInterval != lastWrittenRuntimeInterval {
						if err := retry.Do(func() error {
							ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
							defer cancel()
							fields := map[string]interface{}{
								"temperature":          currentTemp.Unwrap(),
								"temperature_f":        currentTemp.Unwrap(),
								"temperature_c":        currentTemp.C().Unwrap(),
								"humidity":             currentHumidity,
								"heat_set_point":       heatSetPoint.Unwrap(),
								"heat_set_point_f":     heatSetPoint.Unwrap(),
								"heat_set_point_c":     heatSetPoint.C().Unwrap(),
								"cool_set_point":       coolSetPoint.Unwrap(),
								"cool_set_point_f":     coolSetPoint.Unwrap(),
								"cool_set_point_c":     coolSetPoint.C().Unwrap(),
								"demand_mgmt_offset":   demandMgmtOffset.Unwrap(),
								"demand_mgmt_offset_f": demandMgmtOffset.Unwrap(),
								"demand_mgmt_offset_c": demandMgmtOffset.C().Unwrap(),
								"fan_run_time":         fanRunSec,
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
							if influxEnabled {
								err := influxWriteAPI.WritePoint(ctx,
									influxdb2.NewPoint(
										"ecobee_runtime",
										map[string]string{thermostatNameTag: t.Name},
										fields,
										reportTime,
									))
								if err != nil {
									return err
								}
							}

							// Publish to MQTT if enabled
							if config.MQTT.Enabled && mqttClient != nil {
								// Publish runtime data
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/temperature_f", currentTemp.Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/temperature_c", currentTemp.C().Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/humidity", currentHumidity)
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/heat_set_point_f", heatSetPoint.Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/heat_set_point_c", heatSetPoint.C().Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/cool_set_point_f", coolSetPoint.Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/cool_set_point_c", coolSetPoint.C().Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/demand_mgmt_offset_f", demandMgmtOffset.Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/demand_mgmt_offset_c", demandMgmtOffset.C().Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/fan_run_time", fanRunSec)

								if config.WriteHumidifier || config.WriteDehumidifier {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/humidity_set_point", humiditySetPoint)
								}
								if config.WriteHumidifier {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/humidifier_run_time", humidifierRunSec)
								}
								if config.WriteDehumidifier {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/dehumidifier_run_time", dehumidifierRunSec)
								}
								if config.WriteAuxHeat1 {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/aux_heat_1_run_time", auxHeat1RunSec)
								}
								if config.WriteAuxHeat2 {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/aux_heat_2_run_time", auxHeat2RunSec)
								}
								if config.WriteHeatPump1 {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/heat_pump_1_run_time", heatPump1RunSec)
								}
								if config.WriteHeatPump2 {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/heat_pump_2_run_time", heatPump2RunSec)
								}
								if config.WriteCool1 {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/cool_1_run_time", cool1RunSec)
								}
								if config.WriteCool2 {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, "runtime/cool_2_run_time", cool2RunSec)
								}
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
					var temp wx.TempF
					var presence, presenceSupported bool
					for _, c := range sensor.Capability {
						if c.Type == "temperature" {
							tempInt, err := strconv.Atoi(c.Value)
							if err != nil {
								log.Printf("error reading temp '%s' for sensor %s: %s", c.Value, sensor.Name, err)
							} else {
								temp = wx.TempF(float64(tempInt) / 10.0)
							}
						} else if c.Type == "occupancy" {
							presenceSupported = true
							presence = c.Value == "true"
						}
					}
					fmt.Printf("Sensor '%s' at %s:\n", name, sensorTime)
					fmt.Printf("\ttemperature: %.1f degF (%.1f degC)\n", temp, temp.C())
					if presenceSupported {
						fmt.Printf("\toccupied: %t\n", presence)
					}

					if temp == 0.0 {
						// no temp reading from this sensor, so skip writing it to Influx
						continue
					}

					if sensorTime != lastWrittenSensors {
						if err := retry.Do(func() error {
							ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
							defer cancel()
							fields := map[string]interface{}{
								"temperature":   temp.Unwrap(),
								"temperature_f": temp.Unwrap(),
								"temperature_c": temp.C().Unwrap(),
							}
							if presenceSupported {
								fields["occupied"] = presence
							}
							if influxEnabled {
								err := influxWriteAPI.WritePoint(ctx,
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
							}

							// Publish to MQTT if enabled
							if config.MQTT.Enabled && mqttClient != nil {
								sensorPrefix := fmt.Sprintf("sensor/%s", sensor.Name)
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, fmt.Sprintf("%s/temperature_f", sensorPrefix), temp.Unwrap())
								publishToMQTT(mqttClient, config.MQTT.TopicRoot, fmt.Sprintf("%s/temperature_c", sensorPrefix), temp.C().Unwrap())

								if presenceSupported {
									publishToMQTT(mqttClient, config.MQTT.TopicRoot, fmt.Sprintf("%s/occupied", sensorPrefix), presence)
								}
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
				outdoorTemp := wx.TempF(float64(t.Weather.Forecasts[0].Temperature) / 10.0)
				pressureMillibar := wx.PressureMb(t.Weather.Forecasts[0].Pressure)
				outdoorHumidity := wx.ClampedRelHumidity(t.Weather.Forecasts[0].RelativeHumidity)
				dewpoint := wx.TempF(float64(t.Weather.Forecasts[0].Dewpoint) / 10.0)
				windSpeedMph := wx.SpeedMph(t.Weather.Forecasts[0].WindSpeed)
				windBearing := t.Weather.Forecasts[0].WindBearing
				visibilityMeters := wx.Meter(t.Weather.Forecasts[0].Visibility)
				visibilityMiles := visibilityMeters.Miles()
				windChill := wx.WindChillF(outdoorTemp, windSpeedMph)
				weatherSymbol := t.Weather.Forecasts[0].WeatherSymbol
				sky := t.Weather.Forecasts[0].Sky

				fmt.Printf("Weather at %s:\n", weatherTime)
				fmt.Printf("\ttemperature: %.1f degF (%.1f degC)\n\tpressure: %.0f mb\n\thumidity: %d%%\n\tdew point: %.1f degF (%.1f degC)",
					outdoorTemp, outdoorTemp.C(), pressureMillibar, outdoorHumidity, dewpoint, dewpoint.C())
				fmt.Printf("\n\twind: %d at %.0f mph\n\twind chill: %.1f degF\n\tvisibility: %.1f miles\nweather symbol: %d\nsky: %d",
					windBearing, windSpeedMph, windChill, visibilityMiles, weatherSymbol, sky)

				if weatherTime != lastWrittenWeather || config.AlwaysWriteWeather {
					if err := retry.Do(func() error {
						ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
						defer cancel()
						pointTime := weatherTime
						if config.AlwaysWriteWeather {
							pointTime = time.Now()
						}
						if influxEnabled {
							err := influxWriteAPI.WritePoint(ctx,
								influxdb2.NewPoint(
									ecobeeWeatherMeasurementName,
									map[string]string{ // tags
										thermostatNameTag: t.Name,
										sourceTag:         source,
									},
									map[string]interface{}{ // fields
										"outdoor_temp":                    outdoorTemp.Unwrap(),
										"outdoor_temp_f":                  outdoorTemp.Unwrap(),
										"outdoor_temp_c":                  outdoorTemp.C().Unwrap(),
										"outdoor_humidity":                outdoorHumidity.Unwrap(),
										"barometric_pressure_mb":          int(math.Round(pressureMillibar.Unwrap())), // we get int precision from Ecobee, and historically this is written as int
										"barometric_pressure_inHg":        pressureMillibar.InHg().Unwrap(),
										"dew_point":                       dewpoint.Unwrap(),
										"dew_point_f":                     dewpoint.Unwrap(),
										"dew_point_c":                     dewpoint.C().Unwrap(),
										"wind_speed":                      int(math.Round(windSpeedMph.Unwrap())), // we get int precision from Ecobee, and historically this is written as int
										"wind_speed_mph":                  windSpeedMph.Unwrap(),
										"wind_bearing":                    windBearing,
										"visibility_mi":                   visibilityMiles.Unwrap(),
										"visibility_km":                   visibilityMiles.Km().Unwrap(),
										"recommended_max_indoor_humidity": wx.IndoorHumidityRecommendationF(outdoorTemp).Unwrap(),
										"wind_chill_f":                    windChill.Unwrap(),
										"wind_chill_c":                    windChill.C().Unwrap(),
										"weather_symbol":                  weatherSymbol,
										"sky":                             sky,
									},
									pointTime,
								))
							if err != nil {
								return err
							}
						}

						// Publish to MQTT if enabled
						if config.MQTT.Enabled && mqttClient != nil {
							// Publish weather data
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/outdoor_temp_f", outdoorTemp.Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/outdoor_temp_c", outdoorTemp.C().Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/outdoor_humidity", outdoorHumidity.Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/barometric_pressure_mb", int(math.Round(pressureMillibar.Unwrap())))
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/barometric_pressure_inHg", pressureMillibar.InHg().Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/dew_point_f", dewpoint.Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/dew_point_c", dewpoint.C().Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/wind_speed_mph", windSpeedMph.Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/wind_bearing", windBearing)
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/visibility_mi", visibilityMiles.Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/visibility_km", visibilityMiles.Km().Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/recommended_max_indoor_humidity", wx.IndoorHumidityRecommendationF(outdoorTemp).Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/wind_chill_f", windChill.Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/wind_chill_c", windChill.C().Unwrap())
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/weather_symbol", weatherSymbol)
							publishToMQTT(mqttClient, config.MQTT.TopicRoot, "weather/sky", sky)
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
