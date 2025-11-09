/*
 * DotAAS Part 2 | HTTP/REST | Submodel Repository Service Specification
 *
 * The entire Submodel Repository Service Specification as part of the [Specification of the Asset Administration Shell: Part 2](http://industrialdigitaltwin.org/en/content-hub).   Publisher: Industrial Digital Twin Association (IDTA) 2023
 *
 * API version: V3.0.3_SSP-001
 * Contact: info@idtwin.org
 */

package model

import (
	"time"

	"gorm.io/gorm"
)

// Key type of Key
type Key struct {
	DbID      int64          `json:"-" gorm:"column:id;uniqueIndex"`
	CreatedAt time.Time      `json:"-" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"-" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	Type      KeyTypes       `json:"type"`

	Value string `json:"value" validate:"regexp=^([\\\\x09\\\\x0a\\\\x0d\\\\x20-\\\\ud7ff\\\\ue000-\\\\ufffd]|\\\\ud800[\\\\udc00-\\\\udfff]|[\\\\ud801-\\\\udbfe][\\\\udc00-\\\\udfff]|\\\\udbff[\\\\udc00-\\\\udfff])*$"`

	// Foreign key fields for GORM relationships
	ReferenceID             *uint `json:"-" gorm:"index"`
	ReferenceValueID        *uint `json:"-" gorm:"index"`
	ReferenceElementValueID *uint `json:"-" gorm:"index"`
	ReferenceParentID       *uint `json:"-" gorm:"index"`
	SubmodelElementValueID  *uint `json:"-" gorm:"index"`
}

// AssertKeyRequired checks if the required fields are not zero-ed
func AssertKeyRequired(obj Key) error {
	elements := map[string]interface{}{
		"type":  obj.Type,
		"value": obj.Value,
	}
	for name, el := range elements {
		if isZero := IsZeroValue(el); isZero {
			return &RequiredError{Field: name}
		}
	}

	return nil
}

// AssertKeyConstraints checks if the values respects the defined constraints
//
//nolint:all
func AssertKeyConstraints(obj Key) error {
	return nil
}
