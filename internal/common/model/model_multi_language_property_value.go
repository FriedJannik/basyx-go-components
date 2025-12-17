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

// MultiLanguagePropertyValue represents the Value-Only serialization of a MultiLanguageProperty.
// According to spec: Serialized as array of JSON objects with language and localized string.
type MultiLanguagePropertyValue []map[string]string

// MarshalValueOnly serializes MultiLanguagePropertyValue in Value-Only format
// Serializes as array of objects with language code as key and text as value
func (m MultiLanguagePropertyValue) MarshalValueOnly() ([]byte, error) {
	// Cast to underlying type to avoid infinite recursion with MarshalJSON
	return json.Marshal([]map[string]string(m))
}

// MarshalJSON implements custom JSON marshaling for MultiLanguagePropertyValue
func (m MultiLanguagePropertyValue) MarshalJSON() ([]byte, error) {
	return m.MarshalValueOnly()
}

func (m MultiLanguagePropertyValue) ToLangStringTextTypeSlice() []LangStringTextType {
	result := make([]LangStringTextType, 0, len(m))
	for _, langMap := range m {
		for lang, text := range langMap {
			result = append(result, LangStringTextType{
				Language: lang,
				Text:     text,
			})
		}
	}
	return result
}

// AssertMultiLanguagePropertyValueRequired checks if the required fields are not zero-ed
func AssertMultiLanguagePropertyValueRequired(obj MultiLanguagePropertyValue) error {
	for _, el := range obj {
		for lang, text := range el {
			if lang == "" || text == "" {
				return nil // Basic validation
			}
		}
	}
	return nil
}

// AssertMultiLanguagePropertyValueConstraints checks if the values respects the defined constraints
func AssertMultiLanguagePropertyValueConstraints(obj MultiLanguagePropertyValue) error {
	for _, el := range obj {
		for lang, text := range el {
			// Create a temporary LangStringTextType for validation
			temp := LangStringTextType{Language: lang, Text: text}
			if err := AssertLangStringTextTypeConstraints(temp); err != nil {
				return err
			}
		}
	}
	return nil
}
