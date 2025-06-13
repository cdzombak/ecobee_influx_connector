# Ecobee -> InfluxDB/MQTT Connector

Ship your Ecobee runtime, sensor and weather data to InfluxDB and/or MQTT.

## Getting Started

1. Register and enable the developer dashboard on your Ecobee account at https://www.ecobee.com/developers/
2. Go to https://www.ecobee.com/consumerportal/index.html , navigate to Developer in the right-hand menu, and create an App.
3. Create a config.json file similar to config.example.json above. This file should exist where your work_dir is defined.
4. Build the project (see Build section).
5. Run `ecobee_influx_connector -list-thermostats -config $WORK_DIR/config.json` at an interactive terminal; it'll provide a PIN. Make sure to you replace $WORK_DIR with your config path.
6. Go to https://www.ecobee.com/consumerportal/index.html, navigate to My Apps in the right-hand menu, and click Add Application.
7. Paste the PIN there and authorize the app.
8. Return to the `ecobee_influx_connector` CLI and hit Enter.

You should then be presented with a list of thermostats in your Ecobee account, along with their IDs.

## Configure

Configuration is specified in a JSON file. Create a file (based on the template `config.example.json` stored in this repository) and customize it:

- `api_key` is created above in steps 1 & 2.
- `thermostat_id` can be pulled from step 5 above; it's typically your device's serial number.
- `work_dir` is where client credentials, `config.json`, and (yet to be implemented) last-written watermarks are stored.
- Use the `influx_*` config fields to configure the connector to send data to your InfluxDB. If using tokens for bucket authentication, then leave the user and password config fields empty.
- Use the `mqtt` config section to configure the connector to send data to your MQTT broker:
  - `enabled`: Set to `true` to enable MQTT publishing
  - `server`: MQTT broker hostname or IP address
  - `port`: MQTT broker port (typically 1883 for unencrypted, 8883 for TLS)
  - `username` and `password`: Optional credentials for the MQTT broker
  - `topic_root`: Root topic under which all data will be published (e.g., "ecobee")
- Use the `write_*` config fields to tell the connector which pieces of equipment you use.

## Run via Docker or Docker Compose

A Dockerfile is provided. To build your Docker image, `cd` into the project directory and run `docker build -t ecobee_influx_connector .`

A Docker image is also provided that can be configured via environment variables. [View it on Docker Hub](https://hub.docker.com/r/cdzombak/ecobee_influx_connector), or pull it via `docker pull cdzombak/ecobee_influx_connector`.

To use the Docker container make sure the path to the `config.json` is provided as a volume with the path `/config`. This location will also be used to store the refresh token and `config.json`.

### Important

Before building a persistent container, you will want to execute `docker run --rm -it -v $HOME/ecobee:/config cdzombak/ecobee_influx_connector -config "/config/config.json" -list-thermostats` so that you can get your token cached (`/config/ecobee-cred-cache`). This will give you a single key you can then use to authenticate with your ecobee api app. After auth you should see the thermostat_ids listed for all your devices.

If you build a persistent container before performing the above, the initial token request will loop and it will be hard to get the cached token.

### Docker Compose

There is an example `docker-compose.yml` file above. Make sure to modify the volumes section so that it maps your `/config` folder to containers `/config` folder.

Example:

```yaml
volumes:
  - $DOCKERAPPPATH/ecobee_influx_connector:/config
```

### Docker Run

Example:

If using the image you built from Dockerfile, use:

```shell
docker run -d --name ecobeetest --restart=always -v ./config:/config -it ecobee_influx_connector
```

If using the Docker image, use:

```shell
docker run -d --name ecobeetest --restart=always -v ./config:/config -it cdzombak/ecobee_influx_connector:latest
```

## Install on Debian via apt repository

Install my Debian repository if you haven't already:

```shell
sudo apt-get install ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://dist.cdzombak.net/deb.key | sudo gpg --dearmor -o /etc/apt/keyrings/dist-cdzombak-net.gpg
sudo chmod 0644 /etc/apt/keyrings/dist-cdzombak-net.gpg
echo -e "deb [signed-by=/etc/apt/keyrings/dist-cdzombak-net.gpg] https://dist.cdzombak.net/deb/oss any oss\n" | sudo tee -a /etc/apt/sources.list.d/dist-cdzombak-net.list > /dev/null
sudo apt-get update
```

Then install `ecobee_influx_connector` via `apt-get`:

```shell
sudo apt-get install ecobee-influx-connector
```

## Build from source

```shell
make build
```

To cross-compile for eg. Linux/amd64:

```shell
env GOOS=linux GOARCH=amd64 go build -ldflags="-X main.version=$(./.version.sh)" -o ./ecobee_influx_connector .
```

## Run via systemd on Linux

1. Build the `ecobee_influx_connector` binary or install it per the instructions above.
2. Copy it to `/usr/local/bin` or your preferred location.
3. Create a work directory for the connector. (I put this at `$HOME/.ecobee_influx_connector`.)
4. Run `chmod 700 $YOUR_NEW_WORK_DIR`. (For my work directory, I ran `chmod 700 $HOME/.ecobee_influx_connector`.)
5. Create a configuration JSON file, per the Configure instructions above. (I put this at `$HOME/.ecobee_influx_connector/config.json`.)
6. Customize [`ecobee-influx-connector.service`](https://raw.githubusercontent.com/cdzombak/ecobee_influx_connector/main/ecobee-influx-connector.example.service) with your user/group name and the path to your config file.
7. Copy that customized `ecobee-influx-connector.service` to `/etc/systemd/system`.
8. Run `chown root:root /etc/systemd/system/ecobee-influx-connector.service`.
9. Run `systemctl daemon-reload && systemctl enable ecobee-influx-connector.service && systemctl start ecobee-influx-connector.service`.
10. Check the service's status with `systemctl status ecobee-influx-connector.service`.

## MQTT Topic Structure

When MQTT is enabled, the connector publishes data to the following topic structure:

### Runtime Data

Runtime data is published to topics under `<topic_root>/runtime/`:

- `<topic_root>/runtime/temperature` - Current temperature in Fahrenheit
- `<topic_root>/runtime/temperature_c` - Current temperature in Celsius
- `<topic_root>/runtime/humidity` - Current humidity percentage
- `<topic_root>/runtime/heat_set_point` - Heat set point in Fahrenheit
- `<topic_root>/runtime/heat_set_point_c` - Heat set point in Celsius
- `<topic_root>/runtime/cool_set_point` - Cool set point in Fahrenheit
- `<topic_root>/runtime/cool_set_point_c` - Cool set point in Celsius
- `<topic_root>/runtime/demand_mgmt_offset` - Demand management offset in Fahrenheit
- `<topic_root>/runtime/demand_mgmt_offset_c` - Demand management offset in Celsius
- `<topic_root>/runtime/fan_run_time` - Fan runtime in seconds
- `<topic_root>/runtime/humidity_set_point` - Humidity set point (if enabled)
- `<topic_root>/runtime/humidifier_run_time` - Humidifier runtime in seconds (if enabled)
- `<topic_root>/runtime/dehumidifier_run_time` - Dehumidifier runtime in seconds (if enabled)
- `<topic_root>/runtime/aux_heat_1_run_time` - Auxiliary heat 1 runtime in seconds (if enabled)
- `<topic_root>/runtime/aux_heat_2_run_time` - Auxiliary heat 2 runtime in seconds (if enabled)
- `<topic_root>/runtime/heat_pump_1_run_time` - Heat pump 1 runtime in seconds (if enabled)
- `<topic_root>/runtime/heat_pump_2_run_time` - Heat pump 2 runtime in seconds (if enabled)
- `<topic_root>/runtime/cool_1_run_time` - Cool 1 runtime in seconds (if enabled)
- `<topic_root>/runtime/cool_2_run_time` - Cool 2 runtime in seconds (if enabled)

### Sensor Data

Sensor data is published to topics under `<topic_root>/sensor/<sensor_name>/`:

- `<topic_root>/sensor/<sensor_name>/temperature` - Sensor temperature in Fahrenheit
- `<topic_root>/sensor/<sensor_name>/temperature_c` - Sensor temperature in Celsius
- `<topic_root>/sensor/<sensor_name>/occupied` - Sensor occupancy status (if supported)

### Air Quality Data

Air quality data is published to topics under `<topic_root>/sensor/`:

- `<topic_root>/sensor/airquality_accuracy` - Air quality accuracy
- `<topic_root>/sensor/airquality_score` - Air quality score
- `<topic_root>/sensor/co2` - CO2 level
- `<topic_root>/sensor/voc` - VOC level

### Weather Data

Weather data is published to topics under `<topic_root>/weather/`:

- `<topic_root>/weather/outdoor_temp` - Outdoor temperature in Fahrenheit
- `<topic_root>/weather/outdoor_temp_c` - Outdoor temperature in Celsius
- `<topic_root>/weather/outdoor_humidity` - Outdoor humidity percentage
- `<topic_root>/weather/barometric_pressure_mb` - Barometric pressure in millibars
- `<topic_root>/weather/barometric_pressure_inHg` - Barometric pressure in inches of mercury
- `<topic_root>/weather/dew_point` - Dew point in Fahrenheit
- `<topic_root>/weather/dew_point_c` - Dew point in Celsius
- `<topic_root>/weather/wind_speed` - Wind speed in mph (integer)
- `<topic_root>/weather/wind_speed_mph` - Wind speed in mph (float)
- `<topic_root>/weather/wind_bearing` - Wind bearing in degrees
- `<topic_root>/weather/visibility_mi` - Visibility in miles
- `<topic_root>/weather/visibility_km` - Visibility in kilometers
- `<topic_root>/weather/recommended_max_indoor_humidity` - Recommended maximum indoor humidity
- `<topic_root>/weather/wind_chill_f` - Wind chill in Fahrenheit
- `<topic_root>/weather/wind_chill_c` - Wind chill in Celsius
- `<topic_root>/weather/weather_symbol` - Weather symbol code
- `<topic_root>/weather/sky` - Sky condition code

## FAQ

### Does the connector support multiple thermostats?

The connector does not directly support multiple thermostats. To support this use case, I'd recommend running multiple copies of the connector. Each copy will need its own working directory and config file, but you should be able to use the same API key for each connector instance.

(If deploying using the "systemd on Linux" instructions, give each connector's service file a unique name, like `ecobee-influx-connector-1.service`, `ecobee-influx-connector-2.service`, and so on.

## License

Apache 2.0; see `LICENSE` in this repository.

## Author

[Chris Dzombak](https://www.dzombak.com) (GitHub: [@cdzombak](https://github.com/cdzombak)).
