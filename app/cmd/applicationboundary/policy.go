package applicationboundary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/token"
	"io"
	"strings"
)

const MaxPolicyBytes = 1 << 20

// ReadPolicy rejects unknown fields, duplicate keys, trailing JSON, empty policies
// and invalid subjects. A misspelled scope must not silently become a clean audit.
func ReadPolicy(r io.Reader) (Policy, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxPolicyBytes+1))
	if err != nil {
		return Policy{}, err
	}
	if len(data) > MaxPolicyBytes {
		return Policy{}, fmt.Errorf("typed policy exceeds %d bytes", MaxPolicyBytes)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueJSON(d); err != nil {
		return Policy{}, err
	}
	if _, err := d.Token(); err != io.EOF {
		return Policy{}, fmt.Errorf("typed policy has trailing JSON")
	}
	var p Policy
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return Policy{}, fmt.Errorf("typed policy: %w", err)
	}
	return p, p.Validate()
}
func uniqueJSON(d *json.Decoder) error {
	t, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			if !ok || keys[s] {
				return fmt.Errorf("typed policy: duplicate or invalid JSON key")
			}
			keys[s] = true
			if e := uniqueJSON(d); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e := uniqueJSON(d); e != nil {
				return e
			}
		}
	default:
		return fmt.Errorf("typed policy: invalid JSON delimiter")
	}
	_, e = d.Token()
	return e
}
func importIdentity(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "*\\ \t\r\n:#") && !strings.HasPrefix(s, "/") && !strings.Contains(s, "//") && !strings.HasSuffix(s, "/") && !strings.Contains("/"+s+"/", "/../") && !strings.Contains("/"+s+"/", "/./")
}
func symbolValid(s Symbol) bool {
	return importIdentity(s.Package) && token.IsIdentifier(s.Name) && s.Name != "_" && !token.Lookup(s.Name).IsKeyword()
}
func (p Policy) Validate() error {
	if p.SchemaVersion != SchemaVersion || len(p.Factories) == 0 || len(p.Factories) > 128 {
		return fmt.Errorf("typed policy requires schemaVersion 1 and 1..128 factories")
	}
	subjects := map[Symbol]bool{}
	for _, f := range p.Factories {
		if !symbolValid(f.Symbol) || subjects[f.Symbol] {
			return fmt.Errorf("typed policy has invalid/duplicate factory %s", f.Symbol)
		}
		subjects[f.Symbol] = true
		if len(f.AllowedCallers) == 0 {
			return fmt.Errorf("factory %s has no explicit permitted caller packages", f.Symbol)
		}
		callers := map[string]bool{}
		for _, c := range f.AllowedCallers {
			if !importIdentity(c) || callers[c] {
				return fmt.Errorf("factory %s has invalid/duplicate caller package", f.Symbol)
			}
			callers[c] = true
		}
		if len(f.Results)+len(f.Arguments) == 0 {
			return fmt.Errorf("factory %s requires at least one result/argument contract", f.Symbol)
		}
		for _, slots := range [][]Slot{f.Results, f.Arguments} {
			indexes := map[int]bool{}
			for _, s := range slots {
				if s.Index < 0 || indexes[s.Index] || !symbolValid(s.Contract) {
					return fmt.Errorf("factory %s has invalid/duplicate slot", f.Symbol)
				}
				indexes[s.Index] = true
			}
		}
	}
	return nil
}
