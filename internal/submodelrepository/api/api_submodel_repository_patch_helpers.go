package api

import (
	"errors"
	"net/http"
	"reflect"

	"github.com/FriedJannik/aas-go-sdk/jsonization"
	"github.com/FriedJannik/aas-go-sdk/stringification"
	"github.com/FriedJannik/aas-go-sdk/types"
	"github.com/eclipse-basyx/basyx-go-components/internal/common"
	gen "github.com/eclipse-basyx/basyx-go-components/internal/common/model"
)

func submodelValueToAnyMap(value gen.SubmodelValue) map[string]any {
	result := make(map[string]any, len(value))
	for key, val := range value {
		result[key] = val
	}
	return result
}

func applyNullMetadataClearsToSubmodelElement(element types.ISubmodelElement, rawPatch map[string]any) {
	if element == nil {
		return
	}

	for field, value := range rawPatch {
		if value != nil {
			continue
		}

		switch field {
		case "description":
			element.SetDescription([]types.ILangStringTextType{})
		case "displayName":
			element.SetDisplayName([]types.ILangStringNameType{})
		case "embeddedDataSpecifications":
			element.SetEmbeddedDataSpecifications([]types.IEmbeddedDataSpecification{})
		case "supplementalSemanticIds":
			element.SetSupplementalSemanticIDs([]types.IReference{})
		case "qualifiers":
			element.SetQualifiers([]types.IQualifier{})
		case "extensions":
			element.SetExtensions([]types.IExtension{})
		case "semanticId":
			element.SetSemanticID(&types.Reference{})
		case "category":
			element.SetCategory(nil)
		}
	}
}

func buildLimitPtr(limit int32) *int {
	if limit <= 0 {
		return nil
	}

	parsedLimit := int(limit)
	return &parsedLimit
}

func mergeJSONObjects(base map[string]any, patch map[string]any) map[string]any {
	merged := make(map[string]any, len(base))
	for key, value := range base {
		merged[key] = value
	}

	for key, patchValue := range patch {
		if patchValue == nil {
			delete(merged, key)
			continue
		}

		baseValue, baseExists := merged[key]
		baseMap, baseIsMap := baseValue.(map[string]any)
		patchMap, patchIsMap := patchValue.(map[string]any)
		if baseExists && baseIsMap && patchIsMap {
			merged[key] = mergeJSONObjects(baseMap, patchMap)
			continue
		}

		merged[key] = patchValue
	}

	return merged
}

func decodeSubmodelIdentifierOrAPIError(submodelIdentifier string, operation string) (string, gen.ImplResponse, bool) {
	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return "", newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), false
	}

	return decodedSubmodelIdentifier, gen.ImplResponse{}, true
}

func deleteSubmodelElementsIfEmpty(jsonSubmodel map[string]any) {
	rawElements, hasSubmodelElements := jsonSubmodel["submodelElements"]
	if !hasSubmodelElements || rawElements == nil {
		return
	}

	elementsValue := reflect.ValueOf(rawElements)
	if elementsValue.Kind() == reflect.Slice && elementsValue.Len() == 0 {
		delete(jsonSubmodel, "submodelElements")
	}
}

func submodelMetadataToJSONPatch(metadata gen.SubmodelMetadata) (map[string]any, error) {
	patch := map[string]any{}
	modelTypeLiteral, ok := stringification.ModelTypeToString(metadata.ModelType)
	if !ok {
		return nil, errors.New("SMREPO-PATCHSMMETA-INVALIDMODELTYPE Invalid modelType value in metadata payload")
	}
	patch["modelType"] = modelTypeLiteral

	if metadata.ID != "" {
		patch["id"] = metadata.ID
	}
	if metadata.Category != "" {
		patch["category"] = metadata.Category
	}
	if len(metadata.Extensions) > 0 {
		extensions := make([]any, 0, len(metadata.Extensions))
		for _, item := range metadata.Extensions {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			extensions = append(extensions, jsonItem)
		}
		patch["extensions"] = extensions
	}
	if metadata.IdShort != "" {
		patch["idShort"] = metadata.IdShort
	}
	if len(metadata.DisplayName) > 0 {
		displayName := make([]any, 0, len(metadata.DisplayName))
		for _, item := range metadata.DisplayName {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			displayName = append(displayName, jsonItem)
		}
		patch["displayName"] = displayName
	}
	if len(metadata.Description) > 0 {
		description := make([]any, 0, len(metadata.Description))
		for _, item := range metadata.Description {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			description = append(description, jsonItem)
		}
		patch["description"] = description
	}
	if metadata.Administration != nil {
		jsonAdministration, err := jsonization.ToJsonable(metadata.Administration)
		if err != nil {
			return nil, err
		}
		patch["administration"] = jsonAdministration
	}
	if len(metadata.EmbeddedDataSpecifications) > 0 {
		embeddedDataSpecifications := make([]any, 0, len(metadata.EmbeddedDataSpecifications))
		for _, item := range metadata.EmbeddedDataSpecifications {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			embeddedDataSpecifications = append(embeddedDataSpecifications, jsonItem)
		}
		patch["embeddedDataSpecifications"] = embeddedDataSpecifications
	}
	if len(metadata.Qualifiers) > 0 {
		qualifiers := make([]any, 0, len(metadata.Qualifiers))
		for _, item := range metadata.Qualifiers {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			qualifiers = append(qualifiers, jsonItem)
		}
		patch["qualifiers"] = qualifiers
	}
	if metadata.SemanticID != nil {
		jsonSemanticID, err := jsonization.ToJsonable(metadata.SemanticID)
		if err != nil {
			return nil, err
		}
		patch["semanticId"] = jsonSemanticID
	}
	if len(metadata.SupplementalSemanticIds) > 0 {
		supplementalSemanticIds := make([]any, 0, len(metadata.SupplementalSemanticIds))
		for _, item := range metadata.SupplementalSemanticIds {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			supplementalSemanticIds = append(supplementalSemanticIds, jsonItem)
		}
		patch["supplementalSemanticIds"] = supplementalSemanticIds
	}
	if metadata.Kind != 0 {
		kindLiteral, ok := stringification.ModellingKindToString(metadata.Kind)
		if !ok {
			return nil, errors.New("SMREPO-PATCHSMMETA-INVALIDKIND Invalid kind value in metadata payload")
		}
		patch["kind"] = kindLiteral
	}

	return patch, nil
}

func submodelElementMetadataToJSONPatch(metadata gen.SubmodelElementMetadata) (map[string]any, error) {
	patch := map[string]any{}
	modelTypeLiteral, ok := stringification.ModelTypeToString(metadata.ModelType)
	if !ok {
		return nil, errors.New("SMREPO-PATCHSMEMETA-INVALIDMODELTYPE Invalid modelType value in metadata payload")
	}
	patch["modelType"] = modelTypeLiteral

	if metadata.Category != "" {
		patch["category"] = metadata.Category
	}
	if len(metadata.Extensions) > 0 {
		extensions := make([]any, 0, len(metadata.Extensions))
		for _, item := range metadata.Extensions {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			extensions = append(extensions, jsonItem)
		}
		patch["extensions"] = extensions
	}
	if metadata.IdShort != "" {
		patch["idShort"] = metadata.IdShort
	}
	if len(metadata.DisplayName) > 0 {
		displayName := make([]any, 0, len(metadata.DisplayName))
		for _, item := range metadata.DisplayName {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			displayName = append(displayName, jsonItem)
		}
		patch["displayName"] = displayName
	}
	if len(metadata.Description) > 0 {
		description := make([]any, 0, len(metadata.Description))
		for _, item := range metadata.Description {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			description = append(description, jsonItem)
		}
		patch["description"] = description
	}
	if len(metadata.EmbeddedDataSpecifications) > 0 {
		embeddedDataSpecifications := make([]any, 0, len(metadata.EmbeddedDataSpecifications))
		for _, item := range metadata.EmbeddedDataSpecifications {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			embeddedDataSpecifications = append(embeddedDataSpecifications, jsonItem)
		}
		patch["embeddedDataSpecifications"] = embeddedDataSpecifications
	}
	if metadata.SemanticID != nil {
		jsonSemanticID, err := jsonization.ToJsonable(metadata.SemanticID)
		if err != nil {
			return nil, err
		}
		patch["semanticId"] = jsonSemanticID
	}
	if len(metadata.SupplementalSemanticIds) > 0 {
		supplementalSemanticIds := make([]any, 0, len(metadata.SupplementalSemanticIds))
		for _, item := range metadata.SupplementalSemanticIds {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			supplementalSemanticIds = append(supplementalSemanticIds, jsonItem)
		}
		patch["supplementalSemanticIds"] = supplementalSemanticIds
	}
	if len(metadata.Qualifiers) > 0 {
		qualifiers := make([]any, 0, len(metadata.Qualifiers))
		for _, item := range metadata.Qualifiers {
			jsonItem, err := jsonization.ToJsonable(item)
			if err != nil {
				return nil, err
			}
			qualifiers = append(qualifiers, jsonItem)
		}
		patch["qualifiers"] = qualifiers
	}
	if metadata.Kind != 0 {
		kindLiteral, ok := stringification.ModellingKindToString(metadata.Kind)
		if !ok {
			return nil, errors.New("SMREPO-PATCHSMEMETA-INVALIDKIND Invalid kind value in metadata payload")
		}
		patch["kind"] = kindLiteral
	}

	return patch, nil
}
