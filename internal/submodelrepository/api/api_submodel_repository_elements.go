package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/FriedJannik/aas-go-sdk/jsonization"
	"github.com/FriedJannik/aas-go-sdk/types"
	"github.com/eclipse-basyx/basyx-go-components/internal/common"
	gen "github.com/eclipse-basyx/basyx-go-components/internal/common/model"
	auth "github.com/eclipse-basyx/basyx-go-components/internal/common/security"
	openapi "github.com/eclipse-basyx/basyx-go-components/pkg/submodelrepositoryapi"
)

func isRemoteFileURL(fileURL string) bool {
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return false
	}

	return parsedURL.Scheme == "http" || parsedURL.Scheme == "https"
}

// GetAllSubmodelElements retrieves paginated submodel elements for a submodel.
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelElements(ctx context.Context, submodelIdentifier string, limit int32, cursor string, level string, _ /*extent*/ string) (gen.ImplResponse, error) {
	const operation = "GetAllSubmodelElements"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	decodedCursor, decodeErr := decodeOptionalCursor(cursor)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	var limitPtr *int
	if limit > 0 {
		parsedLimit := int(limit)
		limitPtr = &parsedLimit
	}

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	elements, nextCursor, err := s.submodelBackend.GetSubmodelElements(ctx, string(decodedSubmodelIdentifier), limitPtr, decodedCursor, false, level)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElements"), nil
	}

	converted := make([]map[string]any, 0, len(elements))
	for _, element := range elements {
		jsonSubmodelElement, convErr := jsonization.ToJsonable(element)
		if convErr != nil {
			return newAPIErrorResponse(convErr, http.StatusInternalServerError, operation, "ToJsonable"), nil
		}
		converted = append(converted, jsonSubmodelElement)
	}

	encodedNextCursor := ""
	if nextCursor != "" {
		encodedNextCursor = common.EncodeString(nextCursor)
	}

	res := gen.GetSubmodelElementsResult{
		PagingMetadata: gen.PagedResultPagingMetadata{Cursor: encodedNextCursor},
		Result:         converted,
	}

	return gen.Response(http.StatusOK, res), nil
}

// The method validates and persists the provided submodel element data.
//
// Parameters:
//   - ctx: Request context (currently unused)
//   - submodelIdentifier: Base64-encoded identifier of the parent submodel
//   - submodelElement: The submodel element data to create
//
// Returns:
//   - gen.ImplResponse: Response containing the created submodel element (HTTP 201)
//   - error: Error if the creation fails
func (s *SubmodelRepositoryAPIAPIService) PostSubmodelElementSubmodelRepo(ctx context.Context, submodelIdentifier string, submodelElement types.ISubmodelElement) (gen.ImplResponse, error) {
	const operation = "PostSubmodelElementSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	if err := s.submodelBackend.AddSubmodelElement(ctx, decodedSubmodelIdentifier, submodelElement); err != nil {
		if common.IsErrDenied(err) {
			return newAPIErrorResponse(err, http.StatusForbidden, operation, "Denied"), nil
		}
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrConflict(err) {
			return newAPIErrorResponse(err, http.StatusConflict, operation, "Conflict"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "AddSubmodelElement"), nil
	}

	jsonSubmodelElement, jsonErr := jsonization.ToJsonable(submodelElement)
	if jsonErr != nil {
		return newAPIErrorResponse(jsonErr, http.StatusInternalServerError, operation, "ToJsonable"), nil
	}

	return gen.Response(http.StatusCreated, jsonSubmodelElement), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelElementsMetadataSubmodelRepo(ctx context.Context, submodelIdentifier string, limit int32, cursor string) (gen.ImplResponse, error) {
	_ = ctx
	const operation = "GetAllSubmodelElementsMetadataSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	decodedCursor, cursorDecodeErr := decodeOptionalCursor(cursor)
	if cursorDecodeErr != nil {
		return newAPIErrorResponse(cursorDecodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	elements, nextCursor, err := s.submodelBackend.GetSubmodelElements(ctx, decodedSubmodelIdentifier, buildLimitPtr(limit), decodedCursor, false, "")
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElements"), nil
	}

	metadataResult := make([]map[string]any, 0, len(elements))
	for _, element := range elements {
		metadata, conversionErr := toSubmodelElementMetadata(element)
		if conversionErr != nil {
			return newAPIErrorResponse(conversionErr, http.StatusInternalServerError, operation, "ToSubmodelElementMetadata"), nil
		}
		metadataResult = append(metadataResult, metadata)
	}

	res := submodelElementMetadataPageResult{
		PagingMetadata: gen.PagedResultPagingMetadata{Cursor: common.EncodeString(nextCursor)},
		Result:         metadataResult,
	}

	return gen.Response(http.StatusOK, res), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelElementsValueOnlySubmodelRepo(ctx context.Context, submodelIdentifier string, limit int32, cursor string, level string, extent string) (gen.ImplResponse, error) {
	_ = ctx
	_ = extent
	const operation = "GetAllSubmodelElementsValueOnlySubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	decodedCursor, decodeErr := decodeOptionalCursor(cursor)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	var limitPtr *int
	if limit > 0 {
		parsedLimit := int(limit)
		limitPtr = &parsedLimit
	}

	elements, nextCursor, err := s.submodelBackend.GetSubmodelElements(ctx, string(decodedSubmodelIdentifier), limitPtr, decodedCursor, true, level)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElements"), nil
	}

	valueOnlyResults := make([]gen.SubmodelElementValue, 0, len(elements))
	for _, element := range elements {
		valueOnly, convErr := gen.SubmodelElementToValueOnly(element)
		if convErr != nil {
			return newAPIErrorResponse(convErr, http.StatusInternalServerError, operation, "SubmodelElementToValueOnly"), nil
		}
		if valueOnly == nil {
			continue
		}

		idShort := element.IDShort()
		if idShort == nil || *idShort == "" {
			continue
		}

		wrapped := make(gen.SubmodelElementCollectionValue)
		wrapped[*idShort] = valueOnly
		valueOnlyResults = append(valueOnlyResults, wrapped)
	}

	encodedNextCursor := ""
	if nextCursor != "" {
		encodedNextCursor = common.EncodeString(nextCursor)
	}

	res := gen.GetSubmodelElementsValueResult{
		PagingMetadata: gen.PagedResultPagingMetadata{Cursor: encodedNextCursor},
		Result:         valueOnlyResults,
	}

	return gen.Response(http.StatusOK, res), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelElementsReferenceSubmodelRepo(ctx context.Context, submodelIdentifier string, limit int32, cursor string, level string) (gen.ImplResponse, error) {
	_ = ctx
	_ = level
	const operation = "GetAllSubmodelElementsReferenceSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	decodedCursor, cursorDecodeErr := decodeOptionalCursor(cursor)
	if cursorDecodeErr != nil {
		return newAPIErrorResponse(cursorDecodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	references, nextCursor, err := s.submodelBackend.GetSubmodelElementReferences(ctx, decodedSubmodelIdentifier, buildLimitPtr(limit), decodedCursor)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElementReferences"), nil
	}

	jsonableArray := make([]map[string]any, 0, len(references))
	for _, ref := range references {
		jsonRef, convErr := jsonization.ToJsonable(ref)
		if convErr != nil {
			return newAPIErrorResponse(convErr, http.StatusInternalServerError, operation, "ToJsonable"), nil
		}
		jsonableArray = append(jsonableArray, jsonRef)
	}

	res := gen.GetReferencesResult{
		PagingMetadata: gen.PagedResultPagingMetadata{Cursor: common.EncodeString(nextCursor)},
		Result:         jsonableArray,
	}

	return gen.Response(http.StatusOK, res), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelElementsPathSubmodelRepo(ctx context.Context, submodelIdentifier string, limit int32, cursor string, level string) (gen.ImplResponse, error) {
	const operation = "GetAllSubmodelElementsPathSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	decodedCursor, cursorDecodeErr := decodeOptionalCursor(cursor)
	if cursorDecodeErr != nil {
		return newAPIErrorResponse(cursorDecodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	paths, nextCursor, err := s.submodelBackend.GetSubmodelElementPathPage(ctx, decodedSubmodelIdentifier, buildLimitPtr(limit), decodedCursor, level)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElementPathPage"), nil
	}

	res := gen.GetPathItemsResult{
		PagingMetadata: gen.PagedResultPagingMetadata{Cursor: common.EncodeString(nextCursor)},
		Result:         paths,
	}

	return gen.Response(http.StatusOK, res), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelElementByPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, level string, extent string) (gen.ImplResponse, error) {
	_ = extent
	const operation = "GetSubmodelElementByPathSubmodelRepo"

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	element, err := s.submodelBackend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, false, level)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElement"), nil
	}

	converted, convErr := jsonization.ToJsonable(element)
	if convErr != nil {
		return newAPIErrorResponse(convErr, http.StatusInternalServerError, operation, "ToJsonable"), nil
	}

	return gen.Response(http.StatusOK, converted), nil
}

// within submodel elements hierarchy.
//
// Parameters:
//   - ctx: Request context (currently unused)
//   - submodelIdentifier: Base64-encoded identifier of the parent submodel
//   - idShortPath: Path to the target submodel element
//   - submodelElement: Updated submodel element data
//   - level: Detail level for response
//
// Returns:
//   - gen.ImplResponse: HTTP 201 when created, HTTP 204 when updated
//   - error: Error if the operation fails
func (s *SubmodelRepositoryAPIAPIService) PutSubmodelElementByPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, submodelElement types.ISubmodelElement, _ /*level*/ string) (gen.ImplResponse, error) {
	const operation = "PutSubmodelElementByPathSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	isUpdate, err := s.submodelBackend.PutSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, submodelElement)
	if err != nil {
		if common.IsErrDenied(err) {
			return newAPIErrorResponse(err, http.StatusForbidden, operation, "Denied"), nil
		}
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "ParentOrSubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		if common.IsErrConflict(err) {
			return newAPIErrorResponse(err, http.StatusConflict, operation, "Conflict"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "PutSubmodelElement"), nil
	}

	if isUpdate {
		return gen.Response(http.StatusNoContent, nil), nil
	}

	parsedElement, parseErr := jsonization.ToJsonable(submodelElement)
	if parseErr != nil {
		return newAPIErrorResponse(parseErr, http.StatusInternalServerError, operation, "ToJsonable"), nil
	}

	return gen.Response(http.StatusCreated, parsedElement), nil
}

// The method creates a nested element under the specified path.
//
// Parameters:
//   - ctx: Request context (currently unused)
//   - submodelIdentifier: Base64-encoded identifier of the parent submodel
//   - idShortPath: Path where the new element should be created
//   - submodelElement: The submodel element data to create
//
// Returns:
//   - gen.ImplResponse: Response containing the created submodel element (HTTP 201)
//   - error: Error if the creation fails
func (s *SubmodelRepositoryAPIAPIService) PostSubmodelElementByPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, submodelElement types.ISubmodelElement) (gen.ImplResponse, error) {
	const operation = "PostSubmodelElementByPathSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	err := s.submodelBackend.AddSubmodelElementWithPath(ctx, decodedSubmodelIdentifier, idShortPath, submodelElement)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "ParentOrSubmodelNotFound"), nil
		}
		if common.IsErrConflict(err) {
			return newAPIErrorResponse(err, http.StatusConflict, operation, "Conflict"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "AddSubmodelElementWithPath"), nil
	}

	parsedElement, parseErr := jsonization.ToJsonable(submodelElement)
	if parseErr != nil {
		return newAPIErrorResponse(parseErr, http.StatusInternalServerError, operation, "ToJsonable"), nil
	}

	return gen.Response(http.StatusCreated, parsedElement), nil
}

// The method removes the element and all its children from the specified path.
//
// Parameters:
//   - ctx: Request context (currently unused)
//   - submodelIdentifier: Base64-encoded identifier of the parent submodel
//   - idShortPath: Path to the submodel element to delete
//
// Returns:
//   - gen.ImplResponse: Response indicating successful deletion (HTTP 204)
//   - error: Error if the deletion fails
func (s *SubmodelRepositoryAPIAPIService) DeleteSubmodelElementByPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string) (gen.ImplResponse, error) {
	const operation = "DeleteSubmodelElementByPathSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	err := s.submodelBackend.DeleteSubmodelElementByPath(ctx, decodedSubmodelIdentifier, idShortPath)
	if err != nil {
		if common.IsErrDenied(err) {
			return newAPIErrorResponse(err, http.StatusForbidden, operation, "Denied"), nil
		}
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "DeleteSubmodelElementByPath"), nil
	}

	return gen.Response(http.StatusNoContent, nil), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) PatchSubmodelElementByPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, submodelElement types.ISubmodelElement, level string) (gen.ImplResponse, error) {
	const operation = "PatchSubmodelElementByPathSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	if submodelElement == nil {
		return newAPIErrorResponse(errors.New("submodel element payload is required"), http.StatusBadRequest, operation, "MissingSubmodelElementPayload"), nil
	}

	existingElement, getErr := s.submodelBackend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, false, level)
	if getErr != nil {
		if isNotFoundError(getErr) {
			return newAPIErrorResponse(getErr, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}

		return newAPIErrorResponse(getErr, http.StatusInternalServerError, operation, "GetSubmodelElement"), nil
	}

	existingJSON, existingJSONErr := jsonization.ToJsonable(existingElement)
	if existingJSONErr != nil {
		return newAPIErrorResponse(existingJSONErr, http.StatusInternalServerError, operation, "ToJsonableCurrentSubmodelElement"), nil
	}

	patchJSON, patchJSONErr := jsonization.ToJsonable(submodelElement)
	if patchJSONErr != nil {
		return newAPIErrorResponse(patchJSONErr, http.StatusBadRequest, operation, "InvalidSubmodelElementData"), nil
	}

	mergedJSON := mergeJSONObjects(existingJSON, patchJSON)
	if _, hasValuePatch := patchJSON["value"]; !hasValuePatch {
		delete(mergedJSON, "value")
	}

	mergedElement, mergedErr := jsonization.SubmodelElementFromJsonable(mergedJSON)
	if mergedErr != nil {
		return newAPIErrorResponse(mergedErr, http.StatusBadRequest, operation, "InvalidPatchedSubmodelElement"), nil
	}

	err := s.submodelBackend.UpdateSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, mergedElement, false)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		if common.IsErrConflict(err) {
			return newAPIErrorResponse(err, http.StatusConflict, operation, "Conflict"), nil
		}

		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "UpdateSubmodelElement"), nil
	}

	return gen.Response(http.StatusNoContent, nil), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelElementByPathMetadataSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string) (gen.ImplResponse, error) {
	const operation = "GetSubmodelElementByPathMetadataSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	element, err := s.submodelBackend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, false, "")
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElement"), nil
	}

	metadata, conversionErr := toSubmodelElementMetadata(element)
	if conversionErr != nil {
		return newAPIErrorResponse(conversionErr, http.StatusInternalServerError, operation, "ToSubmodelElementMetadata"), nil
	}

	return gen.Response(http.StatusOK, metadata), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) PatchSubmodelElementByPathMetadataSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, submodelElementMetadata gen.SubmodelElementMetadata) (gen.ImplResponse, error) {
	const operation = "PatchSubmodelElementByPathMetadataSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	existingElement, getErr := s.submodelBackend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, false, "")
	if getErr != nil {
		if isNotFoundError(getErr) {
			return newAPIErrorResponse(getErr, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		return newAPIErrorResponse(getErr, http.StatusInternalServerError, operation, "GetSubmodelElement"), nil
	}

	existingJSON, existingJSONErr := jsonization.ToJsonable(existingElement)
	if existingJSONErr != nil {
		return newAPIErrorResponse(existingJSONErr, http.StatusInternalServerError, operation, "ToJsonableCurrentSubmodelElement"), nil
	}

	patchJSON, patchJSONErr := submodelElementMetadataToJSONPatch(submodelElementMetadata)
	if patchJSONErr != nil {
		return newAPIErrorResponse(patchJSONErr, http.StatusBadRequest, operation, "InvalidSubmodelElementMetadata"), nil
	}
	if rawPatchJSON, hasRawPatch := common.GetSubmodelElementMetadataPatch(ctx); hasRawPatch {
		patchJSON = rawPatchJSON
	}

	if existingModelType, ok := existingJSON["modelType"].(string); ok {
		if patchModelType, ok := patchJSON["modelType"].(string); ok && patchModelType != existingModelType {
			mismatchErr := errors.New("metadata modelType must match existing submodel element modelType")
			return newAPIErrorResponse(mismatchErr, http.StatusBadRequest, operation, "InvalidSubmodelElementMetadata"), nil
		}
	}

	mergedJSON := mergeJSONObjects(existingJSON, patchJSON)
	mergedElement, mergedErr := jsonization.SubmodelElementFromJsonable(mergedJSON)
	if mergedErr != nil {
		return newAPIErrorResponse(mergedErr, http.StatusBadRequest, operation, "InvalidPatchedSubmodelElement"), nil
	}
	if rawPatchJSON, hasRawPatch := common.GetSubmodelElementMetadataPatch(ctx); hasRawPatch {
		applyNullMetadataClearsToSubmodelElement(mergedElement, rawPatchJSON)
	}

	if err := s.submodelBackend.UpdateSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, mergedElement, false); err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		if common.IsErrConflict(err) {
			return newAPIErrorResponse(err, http.StatusConflict, operation, "Conflict"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "UpdateSubmodelElement"), nil
	}

	return gen.Response(http.StatusNoContent, nil), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelElementByPathValueOnlySubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, level string, extent string) (gen.ImplResponse, error) {
	_ = extent
	const operation = "GetSubmodelElementByPathValueOnlySubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	element, err := s.submodelBackend.GetSubmodelElement(ctx, string(decodedSubmodelIdentifier), idShortPath, true, level)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElement"), nil
	}

	valueOnly, convErr := gen.SubmodelElementToValueOnly(element)
	if convErr != nil {
		return newAPIErrorResponse(convErr, http.StatusInternalServerError, operation, "SubmodelElementToValueOnly"), nil
	}

	if valueOnly == nil {
		notSerializableErr := errors.New("element cannot be serialized in value-only format")
		return newAPIErrorResponse(notSerializableErr, http.StatusNotFound, operation, "ValueOnlyNotSupported"), nil
	}

	return gen.Response(http.StatusOK, valueOnly), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) PatchSubmodelElementByPathValueOnlySubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, submodelElementValue gen.SubmodelElementValue, level string) (gen.ImplResponse, error) {
	_ = level
	const operation = "PatchSubmodelElementByPathValueOnlySubmodelRepo"

	decodedIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	err := s.submodelBackend.UpdateSubmodelElementValueOnly(ctx, string(decodedIdentifier), idShortPath, submodelElementValue)
	if err != nil {
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		if common.IsErrNotFound(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "UpdateSubmodelElementValueOnly"), nil
	}

	return gen.Response(http.StatusNoContent, nil), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelElementByPathReferenceSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string) (gen.ImplResponse, error) {
	const operation = "GetSubmodelElementByPathReferenceSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	element, err := s.submodelBackend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, false, "")
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElement"), nil
	}

	modelTypeLiteral := getModelTypeLiteral(element)
	if modelTypeLiteral == "" {
		internalErr := common.NewInternalServerError("SMREPO-GETSMEREF-MODELTYPE Empty modelType for submodel element")
		return newAPIErrorResponse(internalErr, http.StatusInternalServerError, operation, "EmptyModelType"), nil
	}

	keyTypes, keyValues, keyResolutionErr := resolveModelReferencePathKeys(
		idShortPath,
		modelTypeLiteral,
		func(path string) (string, error) {
			parentElement, parentErr := s.submodelBackend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, path, false, "")
			if parentErr != nil {
				if common.IsErrBadRequest(parentErr) || common.IsErrNotFound(parentErr) {
					return "", parentErr
				}
				return "", common.NewInternalServerError("SMREPO-BUILDREF-GETPARENT " + parentErr.Error())
			}

			parentModelType := getModelTypeLiteral(parentElement)
			if parentModelType == "" {
				return "", common.NewInternalServerError("SMREPO-BUILDREF-PARENTMODELTYPE Empty modelType for parent submodel element")
			}

			return parentModelType, nil
		},
	)
	if keyResolutionErr != nil {
		if common.IsErrBadRequest(keyResolutionErr) {
			return newAPIErrorResponse(keyResolutionErr, http.StatusBadRequest, operation, "BadReferencePath"), nil
		}
		if common.IsErrNotFound(keyResolutionErr) {
			return newAPIErrorResponse(keyResolutionErr, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		return newAPIErrorResponse(keyResolutionErr, http.StatusInternalServerError, operation, "ResolveReferenceKeys"), nil
	}

	reference, referenceErr := buildModelReference(decodedSubmodelIdentifier, keyTypes, keyValues)
	if referenceErr != nil {
		return newAPIErrorResponse(referenceErr, http.StatusInternalServerError, operation, "BuildModelReference"), nil
	}

	var jsonableRef map[string]any
	if reference != nil {
		jsonableRef, err = jsonization.ToJsonable(reference)
		if err != nil {
			return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ToJsonable"), nil
		}
	}

	return gen.Response(http.StatusOK, jsonableRef), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelElementByPathPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, level string) (gen.ImplResponse, error) {
	const operation = "GetSubmodelElementByPathPathSubmodelRepo"

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	paths, err := s.submodelBackend.GetSubmodelElementPathsByPath(ctx, decodedSubmodelIdentifier, idShortPath, level)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElementPathsByPath"), nil
	}

	return gen.Response(http.StatusOK, paths), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetFileByPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string) (gen.ImplResponse, error) {
	const operation = "GetFileByPathSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	fileSme, err := s.submodelBackend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, false, "")
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElement"), nil
	}

	fileValue, ok := fileSme.(*types.File)
	if !ok {
		notFileErr := common.NewErrMethodNotAllowed("SMREPO-GETFILEBYPATH-NOTFILE Submodel element is not of type File")
		return newAPIErrorResponse(notFileErr, http.StatusMethodNotAllowed, operation, "MethodNotAllowed"), nil
	}

	fileURL := fileValue.Value()
	if fileURL == nil || *fileURL == "" {
		notFoundErr := common.NewErrNotFound("SMREPO-GETFILEBYPATH-EMPTYURL File URL is empty")
		return newAPIErrorResponse(notFoundErr, http.StatusNotFound, operation, "EmptyFileUrl"), nil
	}

	if isRemoteFileURL(*fileURL) {
		return gen.Response(http.StatusFound, openapi.Redirect{Location: *fileURL}), nil
	}

	fileContent, contentType, fileName, err := s.submodelBackend.DownloadFileAttachment(decodedSubmodelIdentifier, idShortPath)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "FileNotFound"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "DownloadFileAttachment"), nil
	}

	return gen.Response(http.StatusOK, openapi.FileDownload{
		Content:     fileContent,
		ContentType: contentType,
		Filename:    fileName,
	}), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) PutFileByPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, fileName string, file *os.File) (gen.ImplResponse, error) {
	const operation = "PutFileByPathSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}
	shouldEnforceExtraSecurityCheck, err := auth.ShouldEnforceFormula(ctx)
	if err != nil {
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ShouldEnforceFormula"), nil
	}

	hasAttachment, err := s.submodelBackend.FileAttachmentExists(decodedSubmodelIdentifier, idShortPath)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrMethodNotAllowed(err) {
			return newAPIErrorResponse(err, http.StatusMethodNotAllowed, operation, "MethodNotAllowed"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ShouldEnforceFormula"), nil
	}

	if shouldEnforceExtraSecurityCheck {

		ctx = auth.SelectPutFormulaByExistence(ctx, hasAttachment)
		_, err = s.submodelBackend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, false, "")
		if err != nil {
			if isNotFoundError(err) || common.IsErrDenied(err) {
				deniedErr := common.NewErrDenied("SMREPO-PUTFILEBYPATH-ABACDENIED writing this file attachment is not allowed")
				return newAPIErrorResponse(deniedErr, http.StatusForbidden, operation, "Denied"), nil
			}
			if common.IsErrBadRequest(err) {
				return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
			}
			return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ShouldEnforceFormula"), nil
		}
	}

	err = s.submodelBackend.UploadFileAttachmentWithHistory(ctx, decodedSubmodelIdentifier, idShortPath, file, fileName)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "UploadFileAttachment"), nil
	}

	return gen.Response(http.StatusNoContent, nil), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) DeleteFileByPathSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string) (gen.ImplResponse, error) {
	const operation = "DeleteFileByPathSubmodelRepo"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	err := s.submodelBackend.DeleteFileAttachmentWithHistory(ctx, decodedSubmodelIdentifier, idShortPath)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "DeleteFileAttachment"), nil
	}

	return gen.Response(http.StatusOK, nil), nil
}
