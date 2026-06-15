package api

import (
	"github.com/FriedJannik/aas-go-sdk/jsonization"
	"github.com/FriedJannik/aas-go-sdk/types"
	gen "github.com/eclipse-basyx/basyx-go-components/internal/common/model"
)

type submodelElementMetadataPageResult struct {
	PagingMetadata gen.PagedResultPagingMetadata `json:"paging_metadata"`
	Result         []map[string]any              `json:"result,omitempty"`
}

func toSubmodelElementMetadata(element types.ISubmodelElement) (map[string]any, error) {
	jsonElement, err := jsonization.ToJsonable(element)
	if err != nil {
		return nil, err
	}

	sanitizeSubmodelElementMetadata(jsonElement)
	return jsonElement, nil
}

func sanitizeSubmodelElementMetadata(metadata map[string]any) {
	modelType, _ := metadata["modelType"].(string)

	switch modelType {
	case "Property", "MultiLanguageProperty", "Blob", "File":
		delete(metadata, "value")
	case "Range":
		delete(metadata, "min")
		delete(metadata, "max")
	case "SubmodelElementCollection", "SubmodelElementList":
		sanitizeSubmodelElementSliceField(metadata, "value")
	case "Operation":
		sanitizeOperationVariables(metadata, "inputVariables")
		sanitizeOperationVariables(metadata, "outputVariables")
		sanitizeOperationVariables(metadata, "inoutputVariables")
	case "Entity":
		sanitizeSubmodelElementSliceField(metadata, "statements")
	case "AnnotatedRelationshipElement":
		sanitizeSubmodelElementSliceField(metadata, "annotations")
	}
}

func sanitizeSubmodelElementSliceField(metadata map[string]any, field string) {
	rawField, exists := metadata[field]
	if !exists {
		return
	}

	rawSlice, ok := rawField.([]any)
	if !ok {
		delete(metadata, field)
		return
	}

	hasSubmodelElements := false
	for _, rawElement := range rawSlice {
		elementMap, mapOK := rawElement.(map[string]any)
		if !mapOK {
			continue
		}

		hasSubmodelElements = true
		sanitizeSubmodelElementMetadata(elementMap)
	}

	if !hasSubmodelElements {
		delete(metadata, field)
	}
}

func sanitizeOperationVariables(metadata map[string]any, field string) {
	rawVariables, exists := metadata[field]
	if !exists {
		return
	}

	variables, ok := rawVariables.([]any)
	if !ok {
		delete(metadata, field)
		return
	}

	for _, rawVariable := range variables {
		variableMap, mapOK := rawVariable.(map[string]any)
		if !mapOK {
			continue
		}

		rawValue, hasValue := variableMap["value"]
		if !hasValue {
			continue
		}

		valueElement, valueOK := rawValue.(map[string]any)
		if !valueOK {
			delete(variableMap, "value")
			continue
		}

		sanitizeSubmodelElementMetadata(valueElement)
	}
}

func getModelTypeLiteral(element types.ISubmodelElement) string {
	jsonElement, err := jsonization.ToJsonable(element)
	if err != nil {
		return ""
	}

	modelType, ok := jsonElement["modelType"].(string)
	if !ok {
		return ""
	}

	return modelType
}
