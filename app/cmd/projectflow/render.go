package projectflow

import (
	"encoding/json"
	"errors"
	"strings"
)

const ReportSchemaVersion = 1

type ReportEnvelope struct {
	SchemaVersion int     `json:"schemaVersion"`
	Command       string  `json:"command"`
	OK            bool    `json:"ok"`
	Stages        []Stage `json:"stages"`
}

func FormatJSON(command string, report Report) ([]byte, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("projectflow: command is required")
	}
	envelope := ReportEnvelope{
		SchemaVersion: ReportSchemaVersion,
		Command:       command,
		OK:            true,
		Stages:        append([]Stage(nil), report.Stages...),
	}
	contents, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}
