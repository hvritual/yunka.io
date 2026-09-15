package advisorcore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// UnmarshalJSON makes a persisted semantic attestation prove that its embedded
// findings are still the exact validated advisor response. The outer
// DecodeSemanticReviewAttestation path separately verifies the attestation
// digest and source identity, so neither self-digest can hide a stale response
// digest after finding content is changed.
func (attestation *SemanticReviewAttestation) UnmarshalJSON(contents []byte) error {
	type plainSemanticReviewAttestation SemanticReviewAttestation
	var decoded plainSemanticReviewAttestation
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("semantic review attestation: trailing JSON value is not allowed")
		}
		return err
	}

	value := SemanticReviewAttestation(decoded)
	response, err := canonicalSemanticResponse(SemanticReviewResponse{
		SchemaVersion: value.SchemaVersion,
		Authority:     value.Authority,
		RequestDigest: value.RequestDigest,
		Findings:      append([]SemanticFinding(nil), value.Findings...),
	})
	if err != nil {
		return fmt.Errorf("semantic review attestation: response projection: %w", err)
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}
	expectedResponseDigest := digest(responseBytes)
	if strings.TrimSpace(value.ResponseDigest) != expectedResponseDigest {
		return fmt.Errorf("semantic review attestation: responseDigest does not match embedded findings")
	}
	*attestation = value
	return nil
}
