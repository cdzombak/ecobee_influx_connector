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

// See
// https://docs.google.com/spreadsheets/d/1y9sjcvV_gTCG4UCVxVP2x6-LpmdunID9_oVMmhctRAI/view#gid=943586157
// for how this file is generated.

type Event struct {
	Type                   string `json:"type"`
	Name                   string `json:"name"`
	Running                bool   `json:"running"`
	StartDate              string `json:"startDate"`
	StartTime              string `json:"startTime"`
	EndDate                string `json:"endDate"`
	EndTime                string `json:"endTime"`
	IsOccupied             bool   `json:"isOccupied"`
	IsCoolOff              bool   `json:"isCoolOff"`
	IsHeatOff              bool   `json:"isHeatOff"`
	CoolHoldTemp           int    `json:"coolHoldTemp"`
	HeatHoldTemp           int    `json:"heatHoldTemp"`
	Fan                    string `json:"fan"`
	Vent                   string `json:"vent,omitempty"`
	VentilatorMinOnTime    int    `json:"ventilatorMinOnTime,omitempty"`
	IsOptional             bool   `json:"isOptional"`
	IsTemperatureRelative  bool   `json:"isTemperatureRelative"`
	CoolRelativeTemp       int    `json:"coolRelativeTemp"`
	HeatRelativeTemp       int    `json:"heatRelativeTemp"`
	IsTemperatureAbsolute  bool   `json:"isTemperatureAbsolute"`
	DutyCyclePercentage    int    `json:"dutyCyclePercentage"`
	FanMinOnTime           int    `json:"fanMinOnTime"`
	OccupiedSensorActive   bool   `json:"occupiedSensorActive,omitempty"`
	UnoccupiedSensorActive bool   `json:"unoccupiedSensorActive"`
	DrRampUpTemp           int    `json:"drRampUpTemp"`
	DrRampUpTime           int    `json:"drRampUpTime"`
	LinkRef                string `json:"linkRef,omitempty"`
	HoldClimateRef         string `json:"holdClimateRef,omitempty"`
}

type SetHoldParams struct {
	Event
	CoolHoldTemp   int    `json:"coolHoldTemp"`
	HeatHoldTemp   int    `json:"heatHoldTemp"`
	HoldClimateRef string `json:"holdClimateRef,omitempty"`
	StartDate      string `json:"startDate,omitempty"`
	StartTime      string `json:"startTime,omitempty"`
	EndDate        string `json:"endDate,omitempty"`
	EndTime        string `json:"endTime,omitempty"`
	HoldType       string `json:"holdType,omitempty"`
	HoldHours      int    `json:"holdHours,omitempty"`
}

type Alert struct {
	//	AcknowledgeRef       string `json:"acknowledgeRef"`
	//	Date                 string `json:"date"`
	//	Time                 string `json:"time"`
	//	Severity             string `json:"severity"`
	Text string `json:"text"`
	//	AlertNumber          int    `json:"alertNumber"`
	AlertType       string `json:"alertType"`
	IsOperatorAlert bool   `json:"isOperatorAlert"`
	// Reminder             string `json:"reminder"`
	// ShowIdt              bool   `json:"showIdt"`
	// ShowWeb              bool   `json:"showWeb"`
	// SendEmail            bool   `json:"sendEmail"`
	// Acknowledgement      string `json:"acknowledgement"`
	// RemindMeLater        bool   `json:"remindMeLater"`
	// ThermostatIdentifier string `json:"thermostatIdentifier"`
	// NotificationType     string `json:"notificationType"`
}

type SendMessageParams struct {
	Alert
	Text string `json:"text"`
}

type Selection struct {
	SelectionType               string `json:"selectionType"`
	SelectionMatch              string `json:"selectionMatch"`
	IncludeRuntime              bool   `json:"includeRuntime"`
	IncludeExtendedRuntime      bool   `json:"includeExtendedRuntime"`
	IncludeElectricity          bool   `json:"includeElectricity"`
	IncludeSettings             bool   `json:"includeSettings"`
	IncludeLocation             bool   `json:"includeLocation"`
	IncludeProgram              bool   `json:"includeProgram"`
	IncludeEvents               bool   `json:"includeEvents"`
	IncludeDevice               bool   `json:"includeDevice"`
	IncludeTechnician           bool   `json:"includeTechnician"`
	IncludeUtility              bool   `json:"includeUtility"`
	IncludeManagement           bool   `json:"includeManagement"`
	IncludeAlerts               bool   `json:"includeAlerts"`
	IncludeWeather              bool   `json:"includeWeather"`
	IncludeHouseDetails         bool   `json:"includeHouseDetails"`
	IncludeOemCfg               bool   `json:"includeOemCfg"`
	IncludeEquipmentStatus      bool   `json:"includeEquipmentStatus"`
	IncludeNotificationSettings bool   `json:"includeNotificationSettings"`
	IncludePrivacy              bool   `json:"includePrivacy"`
	IncludeVersion              bool   `json:"includeVersion"`
	IncludeSecuritySettings     bool   `json:"includeSecuritySettings"`
	IncludeSensors              bool   `json:"includeSensors"`
	IncludeAudio                bool   `json:"includeAudio"`
}

type Function struct {
	Type   string      `json:"type"`
	Params interface{} `json:"params"`
}

type UpdateThermostatRequest struct {
	Selection Selection  `json:"selection"`
	Functions []Function `json:"functions"`
}

type UpdateThermostatResponse struct {
	Status Status `json:"status"`
}

type ResumeProgramParams struct {
	ResumeAll bool `json:"resumeAll"`
}

type Status struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Thermostat struct {
	Identifier     string `json:"identifier"`
	Name           string `json:"name"`
	ThermostatRev  string `json:"thermostatRev"`
	IsRegistered   bool   `json:"isRegistered"`
	ModelNumber    string `json:"modelNumber"`
	Brand          string `json:"brand"`
	Features       string `json:"features"`
	LastModified   string `json:"lastModified"`
	ThermostatTime string `json:"thermostatTime"`
	UtcTime        string `json:"utcTime"`
	//Alerts         []Alert  `json:"alerts"`
	//Settings       Settings `json:"settings"`
	Runtime         Runtime         `json:"runtime"`
	ExtendedRuntime ExtendedRuntime `json:"extendedRuntime"`
	/// ...
	Events  []Event `json:"events"`
	Program Program `json:"program"`
	/// ...
	RemoteSensors []RemoteSensor `json:"remoteSensors"`
	Weather       Weather        `json:"weather"`
}

type Runtime struct {
	RuntimeRev         string `json:"runtimeRev"`
	Connected          bool   `json:"connected"`
	FirstConnected     string `json:"firstConnected"`
	ConnectDateTime    string `json:"connectDateTime"`
	DisconnectDateTime string `json:"disconnectDateTime"`
	LastModified       string `json:"lastModified"`
	LastStatusModified string `json:"lastStatusModified"`
	RuntimeDate        string `json:"runtimeDate"`
	RuntimeInterval    int    `json:"runtimeInterval"`
	ActualTemperature  int    `json:"actualTemperature"`
	ActualHumidity     int    `json:"actualHumidity"`
	DesiredHeat        int    `json:"desiredHeat"`
	DesiredCool        int    `json:"desiredCool"`
	DesiredHumidity    int    `json:"desiredHumidity"`
	DesiredDehumidity  int    `json:"desiredDehumidity"`
	DesiredFanMode     string `json:"desiredFanMode"`
	DesiredHeatRange   []int  `json:"desiredHeatRange"`
	DesiredCoolRange   []int  `json:"desiredCoolRange"`
}

type ExtendedRuntime struct {
	LastReadingTimestamp     string   `json:"lastReadingTimestamp"`
	RuntimeDate              string   `json:"runtimeDate"`
	RuntimeInterval          int      `json:"runtimeInterval"`
	ActualTemperature        []int    `json:"actualTemperature"`
	ActualHumidity           []int    `json:"actualHumidity"`
	DesiredHeat              []int    `json:"desiredHeat"`
	DesiredCool              []int    `json:"desiredCool"`
	DesiredHumidity          []int    `json:"desiredHumidity"`
	DesiredDehumidity        []int    `json:"desiredDehumidity"`
	DmOffset                 []int    `json:"dmOffset"`
	HvacMode                 []string `json:"hvacMode"`
	HeatPump1                []int    `json:"heatPump1"`
	HeatPump2                []int    `json:"heatPump2"`
	AuxHeat1                 []int    `json:"auxHeat1"`
	AuxHeat2                 []int    `json:"auxHeat2"`
	AuxHeat3                 []int    `json:"auxHeat3"`
	Cool1                    []int    `json:"cool1"`
	Cool2                    []int    `json:"cool2"`
	Fan                      []int    `json:"fan"`
	Humidifier               []int    `json:"humidifier"`
	Dehumidifier             []int    `json:"dehumidifier"`
	Economizer               []int    `json:"economizer"`
	Ventilator               []int    `json:"ventilator"`
	CurrentElectricityBill   int      `json:"currentElectricityBill"`
	ProjectedElectricityBill int      `json:"projectedElectricityBill"`
}

type GetThermostatsRequest struct {
	Selection Selection `json:"selection"`
	Page      Page      `json:"page,omitempty"`
}

type GetThermostatsResponse struct {
	Page           Page
	ThermostatList []Thermostat `json:"thermostatList"`
	Status         Status       `json:"status"`
}

type Page struct {
	Page       int `json:"page,omitempty"`
	TotalPages int `json:"totalPages"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
}

type RemoteSensor struct {
	ID         string                   `json:"id"`
	Name       string                   `json:"name"`
	Type       string                   `json:"type"`
	Code       string                   `json:"code"`
	InUse      bool                     `json:"inUse"`
	Capability []RemoteSensorCapability `json:"capability"`
}

type RemoteSensorCapability struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Climate struct {
	Name                string         `json:"name"`
	ClimateRef          string         `json:"climateRef"`
	IsOccupied          bool           `json:"isOccupied"`
	IsOptimized         bool           `json:"isOptimized"`
	CoolFan             string         `json:"coolFan"`
	HeatFan             string         `json:"heatFan"`
	Vent                string         `json:"vent"`
	VentilatorMinOnTime int            `json:"ventilatorMinOnTime"`
	Owner               string         `json:"owner"`
	Type                string         `json:"type"`
	Colour              int            `json:"colour"`
	CoolTemp            int            `json:"coolTemp"`
	HeatTemp            int            `json:"heatTemp"`
	Sensors             []RemoteSensor `json:"sensors"`
}

type Program struct {
	Schedule          [][]string `json:"schedule"`
	Climates          []Climate  `json:"climates"`
	CurrentClimateRef string     `json:"currentClimateRef"`
}

type GetThermostatSummaryRequest struct {
	Selection Selection `json:"selection"`
}

type GetThermostatSummaryResponse struct {
	RevisionList    []string `json:"revisionList"`
	ThermostatCount int      `json:"thermostatCount"`
	StatusList      []string `json:"statusList"`
	Status          Status   `json:"status"`
}

// Not part of the API
type EquipmentStatus struct {
	HeatPump, HeatPump2, HeatPump3, CompCool1, CompCool2, AuxHeat1, AuxHeat2, AuxHeat3, Fan, Humidifier, Dehumidifier, Ventilator, Economizer, CompHotWater, AuxHotWater bool
}

type ThermostatSummary struct {
	Identifier         string `json:"Identifier"`
	Name               string `json:"Name"`
	Connected          bool   `json:"Connected"`
	ThermostatRevision string `json:"ThermostatRevision"`
	AlertsRevision     string `json:"AlertsRevision"`
	RuntimeRevision    string `json:"RuntimeRevision"`
	IntervalRevision   string `json:"IntervalRevision"`
	EquipmentStatus
}
type ThermostatSummaryMap map[string]ThermostatSummary

type Weather struct {
	Timestamp      string            `json:"timestamp"`
	WeatherStation string            `json:"weatherStation"`
	Forecasts      []WeatherForecast `json:"forecasts"`
}

type WeatherForecast struct {
	WeatherSymbol    int    `json:"weatherSymbol"`
	DateTime         string `json:"dateTime"`
	Condition        string `json:"condition"`
	Temperature      int    `json:"temperature"`
	Pressure         int    `json:"pressure"`
	RelativeHumidity int    `json:"relativeHumidity"`
	Dewpoint         int    `json:"dewpoint"`
	Visibility       int    `json:"visibility"`
	WindSpeed        int    `json:"windSpeed"`
	WindGust         int    `json:"windGust"`
	WindDirection    string `json:"windDirection"`
	WindBearing      int    `json:"windBearing"`
	Pop              int    `json:"pop"`
	TempHigh         int    `json:"tempHigh"`
	TempLow          int    `json:"tempLow"`
	Sky              int    `json:"sky"`
}
