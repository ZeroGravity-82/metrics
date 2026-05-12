package config

import (
	"encoding/json"
	"fmt"
	"time"
)

type configDuration time.Duration

func (d *configDuration) UnmarshalJSON(data []byte) error {
	var durationString string
	if err := json.Unmarshal(data, &durationString); err == nil {
		parsedDuration, parseErr := time.ParseDuration(durationString)
		if parseErr != nil {
			return fmt.Errorf("failed to parse duration %q: %w", durationString, parseErr)
		}
		*d = configDuration(parsedDuration)
		return nil
	}
	return fmt.Errorf("duration value must be a string like \"1s\", got %q", durationString)
}
