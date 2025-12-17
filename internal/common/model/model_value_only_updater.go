/*******************************************************************************
* Copyright (C) 2025 the Eclipse BaSyx Authors and Fraunhofer IESE
*
* Permission is hereby granted, free of charge, to any person obtaining
* a copy of this software and associated documentation files (the
* "Software"), to deal in the Software without restriction, including
* without limitation the rights to use, copy, modify, merge, publish,
* distribute, sublicense, and/or sell copies of the Software, and to
* permit persons to whom the Software is furnished to do so, subject to
* the following conditions:
*
* The above copyright notice and this permission notice shall be
* included in all copies or substantial portions of the Software.
*
* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
* EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
* MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
* NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
* LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
* OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
* WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*
* SPDX-License-Identifier: MIT
******************************************************************************/

package model

import "fmt"

// ApplyValueOnlyUpdate updates the value fields of a SubmodelElement based on value-only data.
// The metadata (idShort, semanticId, etc.) is preserved.
func ApplyValueOnlyUpdate(element SubmodelElement, valueOnlyData SubmodelElementValue) error {
	switch e := element.(type) {
	case *Property:
		propValue, ok := valueOnlyData.(PropertyValue)
		if !ok {
			return fmt.Errorf("expected PropertyValue, got %T", valueOnlyData)
		}
		e.Value = propValue.Value
		return nil

	case *MultiLanguageProperty:
		mlpValue, ok := valueOnlyData.(MultiLanguagePropertyValue)
		if !ok {
			return fmt.Errorf("expected MultiLanguagePropertyValue, got %T", valueOnlyData)
		}
		e.Value = []LangStringTextType(mlpValue)
		return nil

	case *Range:
		rangeValue, ok := valueOnlyData.(RangeValue)
		if !ok {
			return fmt.Errorf("expected RangeValue, got %T", valueOnlyData)
		}
		e.Min = rangeValue.Min
		e.Max = rangeValue.Max
		return nil

	case *File:
		fileValue, ok := valueOnlyData.(FileValue)
		if !ok {
			return fmt.Errorf("expected FileValue, got %T", valueOnlyData)
		}
		e.ContentType = fileValue.ContentType
		e.Value = fileValue.Value
		return nil

	case *Blob:
		blobValue, ok := valueOnlyData.(BlobValue)
		if !ok {
			return fmt.Errorf("expected BlobValue, got %T", valueOnlyData)
		}
		e.ContentType = blobValue.ContentType
		e.Value = blobValue.Value
		return nil

	case *ReferenceElement:
		refValue, ok := valueOnlyData.(ReferenceElementValue)
		if !ok {
			return fmt.Errorf("expected ReferenceElementValue, got %T", valueOnlyData)
		}
		e.Value = &Reference{
			Type: refValue.Type,
			Keys: refValue.Keys,
		}
		return nil

	case *RelationshipElement:
		relValue, ok := valueOnlyData.(RelationshipElementValue)
		if !ok {
			return fmt.Errorf("expected RelationshipElementValue, got %T", valueOnlyData)
		}
		e.First = &Reference{
			Type: relValue.First.Type,
			Keys: relValue.First.Keys,
		}
		e.Second = &Reference{
			Type: relValue.Second.Type,
			Keys: relValue.Second.Keys,
		}
		return nil

	case *AnnotatedRelationshipElement:
		areValue, ok := valueOnlyData.(AnnotatedRelationshipElementValue)
		if !ok {
			return fmt.Errorf("expected AnnotatedRelationshipElementValue, got %T", valueOnlyData)
		}
		e.First = &Reference{
			Type: areValue.First.Type,
			Keys: areValue.First.Keys,
		}
		e.Second = &Reference{
			Type: areValue.Second.Type,
			Keys: areValue.Second.Keys,
		}

		// Update annotations
		if areValue.Annotations != nil {
			// Clear existing annotations and add new ones
			newAnnotations := make([]SubmodelElement, 0, len(areValue.Annotations))
			for idShort, annotValue := range areValue.Annotations {
				// Create a simple Property for each annotation
				prop := &Property{
					IdShort: idShort,
					Value:   fmt.Sprintf("%v", annotValue),
				}
				newAnnotations = append(newAnnotations, prop)
			}
			e.Annotations = newAnnotations
		}
		return nil

	case *Entity:
		entityValue, ok := valueOnlyData.(EntityValue)
		if !ok {
			return fmt.Errorf("expected EntityValue, got %T", valueOnlyData)
		}
		if entityValue.EntityType != "" {
			e.EntityType = EntityType(entityValue.EntityType)
		}
		if entityValue.GlobalAssetID != "" {
			e.GlobalAssetID = entityValue.GlobalAssetID
		}
		if entityValue.SpecificAssetIds != nil {
			// Convert []map[string]interface{} to []SpecificAssetID
			specificAssetIds := make([]SpecificAssetID, 0, len(entityValue.SpecificAssetIds))
			for _, assetIDMap := range entityValue.SpecificAssetIds {
				assetID := SpecificAssetID{}
				if name, ok := assetIDMap["name"].(string); ok {
					assetID.Name = name
				}
				if value, ok := assetIDMap["value"].(string); ok {
					assetID.Value = value
				}
				specificAssetIds = append(specificAssetIds, assetID)
			}
			e.SpecificAssetIds = specificAssetIds
		}

		// Update statements
		if entityValue.Statements != nil {
			// Clear existing statements and add new ones
			newStatements := make([]SubmodelElement, 0, len(entityValue.Statements))
			for idShort, stmtValue := range entityValue.Statements {
				// Create a simple Property for each statement
				prop := &Property{
					IdShort: idShort,
					Value:   fmt.Sprintf("%v", stmtValue),
				}
				newStatements = append(newStatements, prop)
			}
			e.Statements = newStatements
		}
		return nil

	case *BasicEventElement:
		beeValue, ok := valueOnlyData.(BasicEventElementValue)
		if !ok {
			return fmt.Errorf("expected BasicEventElementValue, got %T", valueOnlyData)
		}
		e.Observed = &Reference{
			Type: beeValue.Observed.Type,
			Keys: beeValue.Observed.Keys,
		}
		return nil

	case *SubmodelElementCollection:
		secValue, ok := valueOnlyData.(SubmodelElementCollectionValue)
		if !ok {
			return fmt.Errorf("expected SubmodelElementCollectionValue, got %T", valueOnlyData)
		}

		// Update each child element by idShort
		for _, child := range e.Value {
			childIdShort := child.GetIdShort()
			if childIdShort == "" {
				continue
			}
			if childValue, exists := secValue[childIdShort]; exists {
				if err := ApplyValueOnlyUpdate(child, childValue); err != nil {
					return fmt.Errorf("failed to update child '%s': %w", childIdShort, err)
				}
			}
		}
		return nil

	case *SubmodelElementList:
		selValue, ok := valueOnlyData.(SubmodelElementListValue)
		if !ok {
			return fmt.Errorf("expected SubmodelElementListValue, got %T", valueOnlyData)
		}

		// Update each element by index
		if len(selValue) != len(e.Value) {
			return fmt.Errorf("value-only list has %d elements but existing list has %d elements", len(selValue), len(e.Value))
		}

		for i, elementValue := range selValue {
			if err := ApplyValueOnlyUpdate(e.Value[i], elementValue); err != nil {
				return fmt.Errorf("failed to update element at index %d: %w", i, err)
			}
		}
		return nil

	case *Operation:
		// Operations don't have a value field that can be updated via value-only
		// The specification indicates operations have a reference representation
		// We can't really update the operation itself, so we'll return an error
		return fmt.Errorf("operations cannot be updated via value-only representation")

	case *Capability:
		// Capabilities don't have value-only representation
		return fmt.Errorf("capabilities cannot be updated via value-only representation")

	default:
		return fmt.Errorf("unsupported element type: %T", element)
	}
}
