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
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"gorm.io/gorm"
)

// Submodel struct representing a Submodel.
type Submodel struct {
	ID        string         `json:"id" gorm:"primaryKey;column:submodel_id" validate:"regexp=^([\\\\x09\\\\x0a\\\\x0d\\\\x20-\\\\ud7ff\\\\ue000-\\\\ufffd]|\\\\ud800[\\\\udc00-\\\\udfff]|[\\\\ud801-\\\\udbfe][\\\\udc00-\\\\udfff]|\\\\udbff[\\\\udc00-\\\\udfff])*$"`
	CreatedAt time.Time      `json:"-" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"-" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	Extension []Extension    `json:"extension,omitempty" gorm:"foreignKey:SubmodelID;references:ID"`

	Category string `json:"category,omitempty" validate:"regexp=^([\\\\x09\\\\x0a\\\\x0d\\\\x20-\\\\ud7ff\\\\ue000-\\\\ufffd]|\\\\ud800[\\\\udc00-\\\\udfff]|[\\\\ud801-\\\\udbfe][\\\\udc00-\\\\udfff]|\\\\udbff[\\\\udc00-\\\\udfff])*$"`

	//nolint:all
	IdShort string `json:"idShort,omitempty"`

	DisplayName []LangStringNameType `json:"displayName,omitempty" gorm:"foreignKey:SubmodelDisplayNameID"`

	Description []LangStringTextType `json:"description,omitempty" gorm:"foreignKey:SubmodelDescriptionID"`

	ModelType string `json:"modelType" validate:"regexp=^Submodel$"`

	Administration *AdministrativeInformation `json:"administration,omitempty" gorm:"type:jsonb;serializer:json"`

	Kind ModellingKind `json:"kind,omitempty"`

	SemanticID *Reference `json:"semanticId,omitempty" gorm:"type:jsonb;serializer:json"`

	//nolint:all
	SupplementalSemanticIds []*Reference `json:"supplementalSemanticIds,omitempty" gorm:"type:jsonb;serializer:json"`

	Qualifier []Qualifier `json:"qualifier,omitempty" gorm:"type:jsonb;serializer:json"`

	EmbeddedDataSpecifications []EmbeddedDataSpecification `json:"embeddedDataSpecifications,omitempty" gorm:"type:jsonb;serializer:json"`

	SubmodelElements SubmodelElementSlice `json:"submodelElements,omitempty" gorm:"type:jsonb;serializer:json"`
}

// SubmodelElementSlice is a custom type to handle JSON marshaling/unmarshaling of SubmodelElement slices
type SubmodelElementSlice []SubmodelElement

// MarshalJSON implements json.Marshaler for SubmodelElementSlice
func (s SubmodelElementSlice) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}
	var jsonLib = jsoniter.ConfigCompatibleWithStandardLibrary
	return jsonLib.Marshal([]SubmodelElement(s))
}

// UnmarshalJSON implements json.Unmarshaler for SubmodelElementSlice
func (s *SubmodelElementSlice) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}

	var rawMessages []json.RawMessage
	var jsonLib = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := jsonLib.Unmarshal(data, &rawMessages); err != nil {
		return err
	}

	elements := make([]SubmodelElement, len(rawMessages))
	for i, raw := range rawMessages {
		elem, err := UnmarshalSubmodelElement(raw)
		if err != nil {
			return err
		}
		elements[i] = elem
	}

	*s = SubmodelElementSlice(elements)
	return nil
}

// Value implements driver.Valuer for GORM to save SubmodelElementSlice to database
func (s SubmodelElementSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	var jsonLib = jsoniter.ConfigCompatibleWithStandardLibrary
	return jsonLib.Marshal(s)
}

// Scan implements sql.Scanner for GORM to load SubmodelElementSlice from database
func (s *SubmodelElementSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan type %T into SubmodelElementSlice", value)
	}

	return s.UnmarshalJSON(data)
}

// UnmarshalJSON implements custom unmarshaling for Submodel to handle polymorphic SubmodelElements
func (s *Submodel) UnmarshalJSON(data []byte) error {
	type Alias Submodel
	aux := &struct {
		SubmodelElements           []json.RawMessage `json:"submodelElements,omitempty"`
		EmbeddedDataSpecifications []json.RawMessage `json:"embeddedDataSpecifications,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	elements := make([]SubmodelElement, len(aux.SubmodelElements))
	for i, raw := range aux.SubmodelElements {
		elem, err := UnmarshalSubmodelElement(raw)
		if err != nil {
			return err
		}
		elements[i] = elem
	}
	s.SubmodelElements = SubmodelElementSlice(elements)

	s.EmbeddedDataSpecifications = make([]EmbeddedDataSpecification, len(aux.EmbeddedDataSpecifications))
	for i, raw := range aux.EmbeddedDataSpecifications {
		var eds EmbeddedDataSpecification
		if err := json.Unmarshal(raw, &eds); err != nil {
			return err
		}
		s.EmbeddedDataSpecifications[i] = eds
	}

	return nil
}

// AssertSubmodelRequired checks if the required fields are not zero-ed
func AssertSubmodelRequired(obj Submodel) error {
	elements := map[string]interface{}{
		"modelType": obj.ModelType,
		"id":        obj.ID,
	}
	for name, el := range elements {
		if isZero := IsZeroValue(el); isZero {
			return &RequiredError{Field: name}
		}
	}

	for _, el := range obj.Extension {
		if err := AssertExtensionRequired(el); err != nil {
			return err
		}
	}
	if err := AssertIdShortRequired(obj.IdShort); err != nil {
		return err
	}
	if obj.DisplayName != nil {
		for _, el := range obj.DisplayName {
			if err := AssertLangStringNameTypeRequired(el); err != nil {
				return err
			}
		}
	}
	if obj.Description != nil {
		for _, el := range obj.Description {
			if err := AssertLangStringTextTypeRequired(el); err != nil {
				return err
			}
		}
	}
	if obj.Administration != nil {
		if err := AssertAdministrativeInformationRequired(*obj.Administration); err != nil {
			return err
		}
	}
	if obj.SemanticID != nil {
		if err := AssertReferenceRequired(*obj.SemanticID); err != nil {
			return err
		}
	}
	for _, el := range obj.SupplementalSemanticIds {
		if el != nil {
			if err := AssertReferenceRequired(*el); err != nil {
				return err
			}
		}
	}
	for _, el := range obj.Qualifier {
		if err := AssertQualifierRequired(el); err != nil {
			return err
		}
	}
	for _, el := range obj.EmbeddedDataSpecifications {
		if err := AssertEmbeddedDataSpecificationRequired(el); err != nil {
			return err
		}
	}
	return nil
}

// AssertSubmodelConstraints checks if the values respects the defined constraints
func AssertSubmodelConstraints(obj Submodel) error {
	for _, el := range obj.Extension {
		if err := AssertExtensionConstraints(el); err != nil {
			return err
		}
	}
	if err := AssertstringConstraints(obj.IdShort); err != nil {
		return err
	}
	if obj.DisplayName != nil {
		for _, el := range obj.DisplayName {
			if err := AssertLangStringNameTypeConstraints(el); err != nil {
				return err
			}
		}
	}
	if obj.Description != nil {
		for _, el := range obj.Description {
			if err := AssertLangStringTextTypeConstraints(el); err != nil {
				return err
			}
		}
	}
	if obj.Administration != nil {
		if err := AssertAdministrativeInformationConstraints(*obj.Administration); err != nil {
			return err
		}
	}
	if obj.SemanticID != nil {
		if err := AssertReferenceConstraints(*obj.SemanticID); err != nil {
			return err
		}
	}
	for _, el := range obj.SupplementalSemanticIds {
		if el != nil {
			if err := AssertReferenceConstraints(*el); err != nil {
				return err
			}
		}
	}
	for _, el := range obj.Qualifier {
		if err := AssertQualifierConstraints(el); err != nil {
			return err
		}
	}
	for _, el := range obj.EmbeddedDataSpecifications {
		if err := AssertEmbeddedDataSpecificationConstraints(el); err != nil {
			return err
		}
	}
	return nil
}
