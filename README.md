# Ecobee -> InfluxDB Connector

# Build

```shell
go build -o ./ecobee_influx_connector .
```

## Getting Started

1. Register and enable the developer dashboard on your Ecobee account at https://www.ecobee.com/developers/
2. Go to https://www.ecobee.com/consumerportal/index.html , navigate to Developer in the right-hand menu, and create an App.
3. Create a JSON config file containing that API key and a working directory.
4. Run `ecobee_influx_connector -list-thermostats` at an interactive terminal; it'll provide a PIN.
5. Go to https://www.ecobee.com/consumerportal/index.html , navigate to My Apps in the right-hand menu, and click Add Application.
6. Paste the PIN there and authorize the app.
7. Return to the `ecobee_influx_connector` CLI and hit Enter.

You should then be presented with a list of thermostats in your Ecobee account, along with their IDs.

# Configure

Configuration is specified in a JSON file. Create a file (based on the template `config.example.json` stored in this repository) and customize it with your Ecobee API key, thermostat ID, and Influx server.

Use the `write_*` config fields to tell the connector which pieces of equipment you use.

The `work_dir` is where client credentials and (in the future) last-written watermarks are stored.

# Install & Run

- [ ] TODO(cdzombak): install/run docs
    - good idea to chmod the work dir 700 and the files inside 600
    - ecobee-influx-connector.service systemd unit is provided and can be customized

## License

Apache 2; see `LICENSE` in this repository.

## Author

[Chris Dzombak](https://www.dzombak.com) (GitHub [@cdzombak](https://github.com/cdzombak)).
