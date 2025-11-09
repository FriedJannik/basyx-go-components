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

// Extension type of Extension
type Extension struct {
	DbID       int64          `json:"-" gorm:"column:id;uniqueIndex"`
	CreatedAt  time.Time      `json:"-" gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `json:"-" gorm:"autoUpdateTime"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
	SemanticID *Reference     `json:"semanticId,omitempty" gorm:"type:jsonb;serializer:json"`

	//nolint:all
	SupplementalSemanticIds []Reference `json:"supplementalSemanticIds,omitempty" gorm:"type:jsonb;serializer:json"`

	Name string `json:"name" validate:"regexp=^([\\\\x09\\\\x0a\\\\x0d\\\\x20-\\\\ud7ff\\\\ue000-\\\\ufffd]|\\\\ud800[\\\\udc00-\\\\udfff]|[\\\\ud801-\\\\udbfe][\\\\udc00-\\\\udfff]|\\\\udbff[\\\\udc00-\\\\udfff])*$"`

	ValueType DataTypeDefXsd `json:"valueType,omitempty"`

	Value string `json:"value,omitempty"`

	RefersTo []Reference `json:"refersTo,omitempty" gorm:"type:jsonb;serializer:json"`

	// Foreign key for parent relationships
	SubmodelID *string `json:"-" gorm:"index"`
}

// AssertExtensionRequired checks if the required fields are not zero-ed
func AssertExtensionRequired(obj Extension) error {
	elements := map[string]interface{}{
		"name": obj.Name,
	}
	for name, el := range elements {
		if isZero := IsZeroValue(el); isZero {
			return &RequiredError{Field: name}
		}
	}
	if obj.SemanticID != nil {
		if err := AssertReferenceRequired(*obj.SemanticID); err != nil {
			return err
		}
	}
	for _, el := range obj.SupplementalSemanticIds {
		if err := AssertReferenceRequired(el); err != nil {
			return err
		}
	}
	for _, el := range obj.RefersTo {
		if err := AssertReferenceRequired(el); err != nil {
			return err
		}
	}
	return nil
}

// AssertExtensionConstraints checks if the values respects the defined constraints
func AssertExtensionConstraints(obj Extension) error {
	if obj.SemanticID != nil {
		if err := AssertReferenceConstraints(*obj.SemanticID); err != nil {
			return err
		}
	}
	for _, el := range obj.SupplementalSemanticIds {
		if err := AssertReferenceConstraints(el); err != nil {
			return err
		}
	}
	for _, el := range obj.RefersTo {
		if err := AssertReferenceConstraints(el); err != nil {
			return err
		}
	}
	return nil
}
