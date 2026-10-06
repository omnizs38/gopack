// Package metadata describes the application embedded in an installer.
package metadata

import (
	"encoding/json"
	"errors"
	"strings"
)

// MetadataFile is the name of the metadata entry inside the payload archive.
const MetadataFile = "__metadata.json"

// Metadata describes the application being installed.
type Metadata struct {
	AppName    string `json:"appName"`
	AppVersion string `json:"appVersion"`
	MainExe    string `json:"mainExe"`
	Publisher  string `json:"publisher,omitempty"`
}

// Validate checks that the metadata is usable for an installation.
func (m Metadata) Validate() error {
	var missing []string
	if strings.TrimSpace(m.AppName) == "" {
		missing = append(missing, "appName")
	}
	if strings.TrimSpace(m.AppVersion) == "" {
		missing = append(missing, "appVersion")
	}
	if strings.TrimSpace(m.MainExe) == "" {
		missing = append(missing, "mainExe")
	}
	if len(missing) > 0 {
		return errors.New("missing required fields: " + strings.Join(missing, ", "))
	}
	return nil
}

// Encode serializes the metadata as JSON.
func (m Metadata) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// Decode parses metadata from JSON.
func Decode(data []byte) (Metadata, error) {
	var m Metadata
	err := json.Unmarshal(data, &m)
	return m, err
}
