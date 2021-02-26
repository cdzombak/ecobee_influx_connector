# Ecobee -> InfluxDB Connector

# TODO

- [ ] TODO(cdzombak): docs
- [ ] TODO(cdzombak): license
- [ ] TODO(cdzombak): store watermarks in working directory and read them on restart

# Build

- [ ] TODO(cdzombak): build docs

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

- [ ] TODO(cdzombak): config docs

# Install & Run

- [ ] TODO(cdzombak): install/run docs
