/*
 * DotAAS Part 2 | HTTP/REST | Submodel Repository Service Specification
 *
 * The entire Submodel Repository Service Specification as part of the [Specification of the Asset Administration Shell: Part 2](http://industrialdigitaltwin.org/en/content-hub).   Publisher: Industrial Digital Twin Association (IDTA) 2023
 *
 * API version: V3.0.3_SSP-001
 * Contact: info@idtwin.org
 */
//nolint:all
package model

import "encoding/json"

// RawValueOnlyData is a wrapper for raw value-only data that implements SubmodelElementValue interface.
// It holds the unmarshaled JSON data as interface{} before being converted to the appropriate typed value.
type RawValueOnlyData struct {
	rawValue interface{}
}

// NewRawValueOnlyData creates a new RawValueOnlyData wrapper
func NewRawValueOnlyData(rawValue interface{}) RawValueOnlyData {
	return RawValueOnlyData{rawValue: rawValue}
}

// GetRawValue returns the raw value
func (r RawValueOnlyData) GetRawValue() interface{} {
	return r.rawValue
}

// MarshalValueOnly implements SubmodelElementValue interface
func (r RawValueOnlyData) MarshalValueOnly() ([]byte, error) {
	return json.Marshal(r.rawValue)
}
