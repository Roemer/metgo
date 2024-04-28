package metgo

import "time"

type cacheMetaData struct {
	Expires      time.Time `json:"expires"`
	LastModified time.Time `json:"lastModified"`
}

type LocationforecastResult struct {
	Type string `json:"type"`

	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`

	Properties struct {
		Meta       Meta         `json:"meta"`
		Timeseries []Timeseries `json:"timeseries"`
	} `json:"properties"`
}

type Meta struct {
	UpdatedAt time.Time `json:"updated_at"`
	Units     struct {
		AirPressureAtSeaLevel    string `json:"air_pressure_at_sea_level"`
		AirTemperature           string `json:"air_temperature"`
		AirTemperatureMax        string `json:"air_temperature_max"`
		AirTemperatureMin        string `json:"air_temperature_min"`
		CloudAreaFraction        string `json:"cloud_area_fraction"`
		CloudAreaFractionHigh    string `json:"cloud_area_fraction_high"`
		CloudAreaFractionLow     string `json:"cloud_area_fraction_low"`
		CloudAreaFractionMedium  string `json:"cloud_area_fraction_medium"`
		DewPointTemperature      string `json:"dew_point_temperature"`
		FogAreaFraction          string `json:"fog_area_fraction"`
		PrecipitationAmount      string `json:"precipitation_amount"`
		RelativeHumidity         string `json:"relative_humidity"`
		UltravioletIndexClearSky string `json:"ultraviolet_index_clear_sky"`
		WindFromDirection        string `json:"wind_from_direction"`
		WindSpeed                string `json:"wind_speed"`
	} `json:"units"`
}

type Timeseries struct {
	Time time.Time `json:"time"`
	Data struct {
		Instant struct {
			Details struct {
				AirPressureAtSeaLevel    float64 `json:"air_pressure_at_sea_level"`
				AirTemperature           float64 `json:"air_temperature"`
				CloudAreaFraction        float64 `json:"cloud_area_fraction"`
				CloudAreaFractionHigh    float64 `json:"cloud_area_fraction_high"`
				CloudAreaFractionLow     float64 `json:"cloud_area_fraction_low"`
				CloudAreaFractionMedium  float64 `json:"cloud_area_fraction_medium"`
				DewPointTemperature      float64 `json:"dew_point_temperature"`
				FogAreaFraction          float64 `json:"fog_area_fraction"`
				RelativeHumidity         float64 `json:"relative_humidity"`
				UltravioletIndexClearSky float64 `json:"ultraviolet_index_clear_sky"`
				WindFromDirection        float64 `json:"wind_from_direction"`
				WindSpeed                float64 `json:"wind_speed"`
			} `json:"details"`
		} `json:"instant"`
		Next1_Hours  *NextXHours `json:"next_1_hours,omitempty"`
		Next6_Hours  *NextXHours `json:"next_6_hours,omitempty"`
		Next12_Hours *NextXHours `json:"next_12_hours,omitempty"`
	} `json:"data"`
}

type NextXHours struct {
	Summary struct {
		SymbolCode string `json:"symbol_code"`
	} `json:"summary"`
	Details struct {
		AirTemperatureMax           float64 `json:"air_temperature_max"`
		AirTemperatureMin           float64 `json:"air_temperature_min"`
		PrecipitationAmount         float64 `json:"precipitation_amount"`
		PrecipitationAmountMax      float64 `json:"precipitation_amount_max"`
		PrecipitationAmountMin      float64 `json:"precipitation_amount_min"`
		ProbabilityOfPrecipitation  float64 `json:"probability_of_precipitation"`
		ProbabilityOfThunder        float64 `json:"probability_of_thunder"`
		UltravioletIndexClearSkyMax float64 `json:"ultraviolet_index_clear_sky_max"`
	} `json:"details"`
}
