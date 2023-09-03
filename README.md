# Ecobee -> InfluxDB Connector

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
- Use the `write_*` config fields to tell the connector which pieces of equipment you use.

## Run via Docker or Docker Compose

A Dockerfile is provided. To build your Docker image, `cd` into the project directory and run `docker build -t ecobee_influx_connector .`

A Docker image is also provided that can be configured via environment variables. [View it on Docker Hub](https://hub.docker.com/r/cdzombak/ecobee_influx_connector), or pull it via `docker pull cdzombak/ecobee_influx_connector`.

To use the Docker container make sure the path to the `config.json` is provided as a volume with the path `/config`. This location will also be used to store the refresh token and `config.json`.

### Important

Before building a persistent container, you will want to execute `docker run --rm -it -v $HOME/ecobee:/config cdzombak/ecobee_influx_connector /go/src/app/ecobee_influx_connector -config "/config/config.json" -list-thermostats` so that you can get your token cached (`/config/ecobee-cred-cache`). This will give you a single key you can then use to authenticate with your ecobee api app. After auth you should see the thermostat_ids listed for all your devices.

If you build a persistent container before performing the above, the initial token request will loop and it will be hard to get the cached token.

### Docker Compose

There is an example `docker-compose.yml` file above. Make sure to modify the volumes section so that it maps your `/config` folder to containers `/config` folder.

Example:

`volumes:`\
`- $DOCKERAPPPATH/ecobee_influx_connector:/config`

### Docker Run

Example: 

If using the image you built from Dockerfile, use:

```shell
docker run -d --name ecobeetest --restart=always -v ./config:/config -it ecobee_influx_connector
`

If using the Docker image, use:

```shell
docker run -d --name ecobeetest --restart=always -v ./config:/config -it cdzombak/ecobee_influx_connector:latest
`

## Build

```shell
go build -o ./ecobee_influx_connector .
```

To cross-compile for eg. Linux/amd64:

```shell
env GOOS=linux GOARCH=amd64 go build -o ./ecobee_influx_connector .
```

## Install & Run via systemd on Linux

1. Build the `ecobee_influx_connector` binary per the Build instructions above.
2. Copy it to `/usr/local/bin` or your preferred location.
3. Create a work directory for the connector. (I put this at `$HOME/.ecobee_influx_connector`.)
4. Run `chmod 700 $YOUR_NEW_WORK_DIR`. (For my work directory, I ran `chmod 700 $HOME/.ecobee_influx_connector`.)
5. Create a configuration JSON file, per the Configure instructions above. (I put this at `$HOME/.ecobee_influx_connector/config.json`.)
6. Customize [`ecobee-influx-connector.service`](https://raw.githubusercontent.com/cdzombak/ecobee_influx_connector/main/ecobee-influx-connector.service) with your user/group name and the path to your config file.
7. Copy that customized `ecobee-influx-connector.service` to `/etc/systemd/system`.
8. Run `chown root:root /etc/systemd/system/ecobee-influx-connector.service`.
9. Run `systemctl daemon-reload && systemctl enable ecobee-influx-connector.service && systemctl start ecobee-influx-connector.service`.
10. Check the service's status with `systemctl status ecobee-influx-connector.service`.

## FAQ

### Does the connector support multiple thermostats?

The connector does not directly support multiple thermostats. To support this use case, I'd recommend running multiple copies of the connector. Each copy will need its own working directory and config file, but you should be able to use the same API key for each connector instance.

(If deploying using the "systemd on Linux" instructions, give each connector's service file a unique name, like `ecobee-influx-connector-1.service`, `ecobee-influx-connector-2.service`, and so on.

## License

Apache 2; see `LICENSE` in this repository.

## Author

[Chris Dzombak](https://www.dzombak.com) (GitHub [@cdzombak](https://github.com/cdzombak)).
