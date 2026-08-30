package assemblyplan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func Load(path string) (Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, err
	}
	return LoadBytes(data)
}

func LoadBytes(data []byte) (Plan, error) {
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, fmt.Errorf("assemblyplan: decode: %w", err)
	}
	plan = Normalize(plan)
	if err := Validate(plan); err != nil {
		return Plan{}, err
	}
	if err := validateCommittedPlan(plan); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func Save(path string, plan Plan) error {
	data, err := CanonicalJSON(plan)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".assembly-plan-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
