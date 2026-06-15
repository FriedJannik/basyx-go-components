package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/FriedJannik/aas-go-sdk/jsonization"
	"github.com/FriedJannik/aas-go-sdk/types"
	"github.com/eclipse-basyx/basyx-go-components/internal/common"
	"github.com/eclipse-basyx/basyx-go-components/internal/common/history"
	gen "github.com/eclipse-basyx/basyx-go-components/internal/common/model"
	"golang.org/x/sync/errgroup"
)

func matchesSubmodelRecentRowSemanticID(row history.Row, semanticID string) (bool, error) {
	if row.Deleted {
		return false, nil
	}
	submodel, err := jsonization.SubmodelFromJsonable(row.Snapshot)
	if err != nil {
		return false, err
	}
	return matchesSubmodelSemanticID(submodel, semanticID), nil
}

func matchesSubmodelSemanticID(submodel types.ISubmodel, semanticID string) bool {
	semanticID = strings.TrimSpace(semanticID)
	if semanticID == "" {
		return true
	}
	semanticReference := submodel.SemanticID()
	if semanticReference == nil {
		return false
	}
	for _, key := range semanticReference.Keys() {
		if key != nil && key.Value() == semanticID {
			return true
		}
	}
	return false
}

// GetAllSubmodels retrieves submodels with optional idShort filtering and cursor pagination.
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodels(
	ctx context.Context,
	_ /*semanticID*/ string,
	idShort string,
	limit int32,
	cursor string,
	level string,
	_ /*extent*/ string,
) (gen.ImplResponse, error) {
	const operation = "GetAllSubmodels"

	decodedCursor, decodeErr := decodeOptionalCursor(cursor)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	sms, nextCursor, err := s.submodelBackend.GetSubmodels(ctx, limit, decodedCursor, idShort)
	if err != nil {
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodels"), nil
	}

	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(8)

	for index := range sms {
		sm := sms[index]

		eg.Go(func() error {
			submodelElements, _, elementsErr := s.submodelBackend.GetSubmodelElements(ctx, sm.ID(), nil, "", false, level)
			if elementsErr != nil {
				return elementsErr
			}

			sm.SetSubmodelElements(submodelElements)
			return nil
		})
	}

	if waitErr := eg.Wait(); waitErr != nil {
		if isNotFoundError(waitErr) {
			return newAPIErrorResponse(waitErr, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		return newAPIErrorResponse(waitErr, http.StatusInternalServerError, operation, "GetSubmodelElements"), nil
	}

	converted := make([]map[string]any, 0, len(sms))

	for _, sm := range sms {
		jsonSubmodel, err := jsonization.ToJsonable(sm)
		if err != nil {
			return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ToJsonable"), nil
		}
		converted = append(converted, jsonSubmodel)
	}

	// using the openAPI provided response struct to include paging metadata
	encodedNextCursor := ""
	if nextCursor != "" {
		encodedNextCursor = common.EncodeString(nextCursor)
	}

	res := gen.GetSubmodelsResult{
		PagingMetadata: gen.PagedResultPagingMetadata{
			Cursor: encodedNextCursor,
		},
		Result: converted,
	}
	return gen.Response(200, res), nil
}

// The method decodes the identifier and fetches the submodel from the repository.
//
// Parameters:
//   - ctx: Request context (currently unused)
//   - id: Base64-encoded submodel identifier
//   - level: Detail level for response (currently unused)
//   - extent: Response extent specification (currently unused)
//
// Returns:
//   - gen.ImplResponse: Response containing the requested submodel
//   - error: Error if the submodel is not found or decoding fails
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelByID(
	ctx context.Context,
	id string,
	level string,
	_ /*extent*/ string,
) (gen.ImplResponse, error) {
	const operation = "GetSubmodelByID"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(id)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	sm, err := s.submodelBackend.GetSubmodelByID(ctx, string(decodedSubmodelIdentifier), level, false)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelByID"), nil
	}
	jsonSubmodel, err := jsonization.ToJsonable(sm)
	if err != nil {
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ToJsonable"), nil
	}
	deleteSubmodelElementsIfEmpty(jsonSubmodel)
	return gen.Response(200, jsonSubmodel), nil
}

// GetSubmodelByIdAndDate returns the Submodel version valid at the requested date.
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelByIdAndDate(
	ctx context.Context,
	id string,
	level string,
	_ string,
	date time.Time,
) (gen.ImplResponse, error) {
	const operation = "GetSubmodelByIdAndDate"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(id)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}
	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	sm, err := s.submodelBackend.GetSubmodelByIDAndDate(ctx, decodedSubmodelIdentifier, date)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelByIDAndDate"), nil
	}

	jsonSubmodel, err := jsonization.ToJsonable(sm)
	if err != nil {
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ToJsonable"), nil
	}
	deleteSubmodelElementsIfEmpty(jsonSubmodel)
	return gen.Response(http.StatusOK, jsonSubmodel), nil
}

// GetSignedSubmodelByID retrieves a signed submodel (JWS compact serialization) by its base64-encoded identifier.
func (s *SubmodelRepositoryAPIAPIService) GetSignedSubmodelByID(
	ctx context.Context,
	id string,
	_ /*level*/ string,
	_ /*extent*/ string,
) (gen.ImplResponse, error) {
	const operation = "GetSignedSubmodelByID"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(id)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	jwsString, err := s.submodelBackend.GetSignedSubmodel(ctx, decodedSubmodelIdentifier)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if err.Error() == "JWS signing not configured: private key not loaded" {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SigningNotConfigured"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSignedSubmodel"), nil
	}

	return gen.Response(http.StatusOK, jwsString), nil
}

// The method decodes the identifier and deletes the corresponding submodel.
//
// Parameters:
//   - ctx: Request context (currently unused)
//   - id: Base64-encoded submodel identifier
//
// Returns:
//   - gen.ImplResponse: Response indicating successful deletion
//   - error: Error if the submodel is not found or deletion fails
func (s *SubmodelRepositoryAPIAPIService) DeleteSubmodelByID(
	ctx context.Context,
	id string,
) (gen.ImplResponse, error) {
	const operation = "DeleteSubmodelByID"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(id)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	err := s.submodelBackend.DeleteSubmodel(ctx, decodedSubmodelIdentifier)
	if err != nil {
		if common.IsErrDenied(err) {
			return newAPIErrorResponse(err, http.StatusForbidden, operation, "Denied"), nil
		}
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "InternalServerError"), nil
	}

	return gen.Response(http.StatusNoContent, nil), nil
}

// The method validates and persists the provided submodel data.
//
// Parameters:
//   - ctx: Request context (currently unused)
//   - submodel: The submodel data to create
//
// Returns:
//   - gen.ImplResponse: Response containing the created submodel (HTTP 201)
//   - error: Error if the creation fails
func (s *SubmodelRepositoryAPIAPIService) PostSubmodel(
	ctx context.Context,
	submodel types.ISubmodel,
) (gen.ImplResponse, error) {
	const operation = "PostSubmodel"

	err := s.submodelBackend.CreateSubmodel(ctx, submodel)

	if err != nil {
		if common.IsErrDenied(err) {
			return newAPIErrorResponse(err, http.StatusForbidden, operation, "Denied"), nil
		}
		if common.IsErrConflict(err) {
			return newAPIErrorResponse(err, http.StatusConflict, operation, "IdConflict"), nil
		}

		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "InvalidSubmodelData"), nil
		}

		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "CreateSubmodel"), nil
	}

	submodelJsonable, err := jsonization.ToJsonable(submodel)
	if err != nil {
		return newAPIErrorResponse(err, http.StatusBadRequest, operation, "InvalidSubmodelData"), nil
	}

	return gen.Response(201, submodelJsonable), nil
}

// It supports idShort and semanticId filtering with cursor-based pagination.
//
// Parameters:
//   - ctx: Request context (currently unused)
//   - semanticID: Semantic identifier for filtering
//   - idShort: Short identifier for filtering
//   - limit: Maximum number of results
//   - cursor: Pagination cursor
//
// Returns:
//   - gen.ImplResponse: Response with submodel metadata
//   - error: Error if metadata retrieval or conversion fails
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelsMetadata(
	ctx context.Context,
	semanticID string,
	idShort string,
	limit int32,
	cursor string) (gen.ImplResponse, error) {
	const operation = "GetAllSubmodelsMetadata"

	decodedCursor, decodeErr := decodeOptionalCursor(cursor)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	submodels, nextCursor, err := s.submodelBackend.GetSubmodels(ctx, limit, decodedCursor, "")
	if err != nil {
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodels"), nil
	}

	idShortFilter := strings.ToLower(strings.TrimSpace(idShort))
	semanticIDFilter := strings.ToLower(strings.TrimSpace(semanticID))

	converted := make([]map[string]any, 0, len(submodels))
	for _, sm := range submodels {
		if sm == nil {
			continue
		}

		if idShortFilter != "" {
			currentIDShort := ""
			if sm.IDShort() != nil {
				currentIDShort = strings.ToLower(*sm.IDShort())
			}
			if !strings.Contains(currentIDShort, idShortFilter) {
				continue
			}
		}

		if semanticIDFilter != "" {
			semanticRef := sm.SemanticID()
			if semanticRef == nil {
				continue
			}

			matchesSemanticID := false
			for _, key := range semanticRef.Keys() {
				if strings.Contains(strings.ToLower(key.Value()), semanticIDFilter) {
					matchesSemanticID = true
					break
				}
			}

			if !matchesSemanticID {
				continue
			}
		}

		jsonSubmodel, convertErr := jsonization.ToJsonable(sm)
		if convertErr != nil {
			return newAPIErrorResponse(convertErr, http.StatusInternalServerError, operation, "ToJsonable"), nil
		}
		delete(jsonSubmodel, "submodelElements")
		converted = append(converted, jsonSubmodel)
	}

	encodedCursor := ""
	if nextCursor != "" {
		encodedCursor = common.EncodeString(nextCursor)
	}

	result := gen.GetSubmodelsMetadataResult{
		PagingMetadata: gen.PagedResultPagingMetadata{Cursor: encodedCursor},
		Result:         converted,
	}

	return gen.Response(http.StatusOK, result), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelsValueOnly(ctx context.Context, semanticID string, idShort string, limit int32, cursor string, level string, extent string) (gen.ImplResponse, error) {
	_ = semanticID
	_ = extent
	const operation = "GetAllSubmodelsValueOnly"

	decodedCursor, decodeErr := decodeOptionalCursor(cursor)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	sms, nextCursor, err := s.submodelBackend.GetSubmodels(ctx, limit, decodedCursor, idShort)
	if err != nil {
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodels"), nil
	}

	valueOnlyResults := make([]map[string]any, len(sms))

	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(8)

	for index := range sms {
		index := index
		sm := sms[index]

		eg.Go(func() error {
			submodelElements, _, elementsErr := s.submodelBackend.GetSubmodelElements(ctx, sm.ID(), nil, "", false, level)
			if elementsErr != nil {
				return elementsErr
			}

			sm.SetSubmodelElements(submodelElements)

			valueOnly, convErr := gen.SubmodelToValueOnly(sm)
			if convErr != nil {
				return convErr
			}

			valueOnlyResults[index] = submodelValueToAnyMap(valueOnly)
			return nil
		})
	}

	if waitErr := eg.Wait(); waitErr != nil {
		if isNotFoundError(waitErr) {
			return newAPIErrorResponse(waitErr, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		return newAPIErrorResponse(waitErr, http.StatusInternalServerError, operation, "GetSubmodelElements"), nil
	}

	encodedNextCursor := ""
	if nextCursor != "" {
		encodedNextCursor = common.EncodeString(nextCursor)
	}

	res := gen.GetSubmodelsValueResult{
		PagingMetadata: gen.PagedResultPagingMetadata{
			Cursor: encodedNextCursor,
		},
		Result: valueOnlyResults,
	}

	return gen.Response(http.StatusOK, res), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelsReference(ctx context.Context, semanticID string, idShort string, limit int32, cursor string, level string) (gen.ImplResponse, error) {
	_ = ctx
	_ = level
	const operation = "GetAllSubmodelsReference"

	decodedCursor, decodeErr := decodeOptionalCursor(cursor)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	decodedSemanticID := ""
	if semanticID != "" {
		decodedSemanticID, decodeErr = common.DecodeString(semanticID)
		if decodeErr != nil {
			return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadSemanticID"), nil
		}
	}

	references, nextCursor, err := s.submodelBackend.GetSubmodelReferences(ctx, limit, decodedCursor, idShort, decodedSemanticID)
	if err != nil {
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelReferences"), nil
	}

	jsonReferences := make([]map[string]any, 0, len(references))
	for _, ref := range references {
		jsonRef, err := jsonization.ToJsonable(ref)
		if err != nil {
			return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ToJsonable"), nil
		}
		jsonReferences = append(jsonReferences, jsonRef)
	}

	res := gen.GetReferencesResult{
		PagingMetadata: gen.PagedResultPagingMetadata{Cursor: common.EncodeString(nextCursor)},
		Result:         jsonReferences,
	}

	return gen.Response(http.StatusOK, res), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelsPath(
	ctx context.Context,
	semanticID string,
	idShort string,
	limit int32,
	cursor string,
	level string,
) (gen.ImplResponse, error) {
	const operation = "GetAllSubmodelsPath"
	if limit < 0 {
		limitErr := common.NewErrBadRequest("SMREPO-GETALLSMPATH-BADLIMIT limit must be >= 0")
		return newAPIErrorResponse(limitErr, http.StatusBadRequest, operation, "BadRequest"), nil
	}

	decodedCursor, decodeErr := decodeOptionalCursor(cursor)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	decodedSemanticID := ""
	if semanticID != "" {
		var decodeErr error
		decodedSemanticID, decodeErr = common.DecodeString(semanticID)
		if decodeErr != nil {
			return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadSemanticID"), nil
		}
	}

	cursorState := decodeAllSubmodelsPathCursorState(decodedCursor)
	if cursorState.PathCursor != "" && cursorState.SubmodelCursor == "" {
		badCursorErr := common.NewErrBadRequest("SMREPO-GETALLSMPATH-BADCURSOR path cursor requires submodel cursor")
		return newAPIErrorResponse(badCursorErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}

	effectiveLimit := int(limit)
	if effectiveLimit == 0 {
		effectiveLimit = 100
	}

	resultPaths := make([]string, 0, effectiveLimit)
	submodelCursor := cursorState.SubmodelCursor
	pathCursor := cursorState.PathCursor
	referencePageLimit := limit
	if referencePageLimit == 0 {
		referencePageLimit = 100
	}

	for len(resultPaths) < effectiveLimit {
		references, nextSubmodelCursor, err := s.submodelBackend.GetSubmodelReferences(ctx, referencePageLimit, submodelCursor, idShort, decodedSemanticID)
		if err != nil {
			if common.IsErrBadRequest(err) {
				return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
			}
			return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelReferences"), nil
		}

		if len(references) == 0 {
			res := gen.GetPathItemsResult{
				PagingMetadata: gen.PagedResultPagingMetadata{
					Cursor: "",
				},
				Result: resultPaths,
			}
			return gen.Response(http.StatusOK, res), nil
		}

		submodelIdentifiers := make([]string, 0, len(references))
		for _, reference := range references {
			submodelIdentifier, extractErr := extractSubmodelIdentifierFromReference(reference)
			if extractErr != nil {
				return newAPIErrorResponse(extractErr, http.StatusInternalServerError, operation, "ExtractSubmodelIdentifier"), nil
			}
			submodelIdentifiers = append(submodelIdentifiers, submodelIdentifier)
		}

		for submodelIndex, submodelIdentifier := range submodelIdentifiers {
			remaining := effectiveLimit - len(resultPaths)

			currentPathCursor := ""
			if submodelIndex == 0 && pathCursor != "" && submodelIdentifier == submodelCursor {
				currentPathCursor = pathCursor
			}

			paths, nextPathCursor, pathErr := s.submodelBackend.GetSubmodelElementPathPage(
				ctx,
				submodelIdentifier,
				&remaining,
				currentPathCursor,
				level,
			)
			if pathErr != nil {
				if isNotFoundError(pathErr) {
					return newAPIErrorResponse(pathErr, http.StatusNotFound, operation, "SubmodelNotFound"), nil
				}
				if common.IsErrBadRequest(pathErr) {
					return newAPIErrorResponse(pathErr, http.StatusBadRequest, operation, "BadRequest"), nil
				}
				return newAPIErrorResponse(pathErr, http.StatusInternalServerError, operation, "GetSubmodelElementPathPage"), nil
			}

			resultPaths = append(resultPaths, paths...)

			if len(resultPaths) == effectiveLimit {
				nextState := allSubmodelsPathCursorState{}
				switch {
				case nextPathCursor != "":
					nextState.SubmodelCursor = submodelIdentifier
					nextState.PathCursor = nextPathCursor
				case submodelIndex+1 < len(submodelIdentifiers):
					nextState.SubmodelCursor = submodelIdentifiers[submodelIndex+1]
				default:
					nextState.SubmodelCursor = nextSubmodelCursor
				}

				encodedCursorState, encodeCursorErr := encodeAllSubmodelsPathCursorState(nextState)
				if encodeCursorErr != nil {
					return newAPIErrorResponse(encodeCursorErr, http.StatusInternalServerError, operation, "EncodeCursor"), nil
				}

				res := gen.GetPathItemsResult{
					PagingMetadata: gen.PagedResultPagingMetadata{
						Cursor: common.EncodeString(encodedCursorState),
					},
					Result: resultPaths,
				}

				return gen.Response(http.StatusOK, res), nil
			}
		}

		if nextSubmodelCursor == "" {
			break
		}

		submodelCursor = nextSubmodelCursor
		pathCursor = ""
	}

	res := gen.GetPathItemsResult{
		PagingMetadata: gen.PagedResultPagingMetadata{
			Cursor: "",
		},
		Result: resultPaths,
	}

	return gen.Response(http.StatusOK, res), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) PutSubmodelByID(ctx context.Context, submodelIdentifier string, submodel types.ISubmodel) (gen.ImplResponse, error) {
	const operation = "PutSubmodelByID"

	decodedIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	if decodedIdentifier != submodel.ID() {
		return newAPIErrorResponse(errors.New("submodel ID in path and body do not match"), http.StatusBadRequest, operation, "IdMismatch"), nil
	}

	isUpdate, err := s.submodelBackend.PutSubmodel(ctx, decodedIdentifier, submodel)
	if err != nil {
		if common.IsErrDenied(err) {
			return newAPIErrorResponse(err, http.StatusForbidden, operation, "Denied"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		if common.IsErrConflict(err) {
			return newAPIErrorResponse(err, http.StatusConflict, operation, "Conflict"), nil
		}
		if common.IsErrNotFound(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "InternalServerError"), nil
	}

	if isUpdate {
		return gen.Response(http.StatusNoContent, nil), nil
	}

	jsonSubmodel, jsonErr := jsonization.ToJsonable(submodel)
	if jsonErr != nil {
		return newAPIErrorResponse(jsonErr, http.StatusBadRequest, operation, "InvalidSubmodelData"), nil
	}

	return gen.Response(http.StatusCreated, jsonSubmodel), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) PatchSubmodelByID(ctx context.Context, submodelIdentifier string, submodel types.ISubmodel, level string) (gen.ImplResponse, error) {
	_ = ctx
	_ = level
	const operation = "PatchSubmodelByID"

	decodedIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	if submodel == nil {
		return newAPIErrorResponse(errors.New("submodel payload is required"), http.StatusBadRequest, operation, "MissingSubmodelPayload"), nil
	}

	if submodel.ID() != "" && decodedIdentifier != submodel.ID() {
		return newAPIErrorResponse(errors.New("submodel ID in path and body do not match"), http.StatusBadRequest, operation, "IdMismatch"), nil
	}

	patchJSON, patchJSONErr := jsonization.ToJsonable(submodel)
	if patchJSONErr != nil {
		return newAPIErrorResponse(patchJSONErr, http.StatusBadRequest, operation, "InvalidSubmodelData"), nil
	}

	_, patchIncludesSubmodelElements := patchJSON["submodelElements"]

	existingSubmodels, _, getErr := s.submodelBackend.GetSubmodels(ctx, 1, "", decodedIdentifier)
	if getErr != nil {
		if isNotFoundError(getErr) {
			return newAPIErrorResponse(getErr, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}

		return newAPIErrorResponse(getErr, http.StatusInternalServerError, operation, "GetSubmodelByID"), nil
	}

	if len(existingSubmodels) == 0 {
		return newAPIErrorResponse(common.NewErrNotFound(decodedIdentifier), http.StatusNotFound, operation, "SubmodelNotFound"), nil
	}

	existingSubmodel := existingSubmodels[0]
	if existingSubmodel == nil {
		nilErr := common.NewInternalServerError("SMREPO-PATCHSM-EXISTINGNIL Existing submodel is nil")
		return newAPIErrorResponse(nilErr, http.StatusInternalServerError, operation, "GetSubmodelByID"), nil
	}

	existingJSON, existingJSONErr := jsonization.ToJsonable(existingSubmodel)
	if existingJSONErr != nil {
		return newAPIErrorResponse(existingJSONErr, http.StatusInternalServerError, operation, "ToJsonableCurrentSubmodel"), nil
	}

	patchJSON["id"] = decodedIdentifier

	mergedJSON := mergeJSONObjects(existingJSON, patchJSON)

	mergedSubmodel, mergedErr := jsonization.SubmodelFromJsonable(mergedJSON)
	if mergedErr != nil {
		return newAPIErrorResponse(mergedErr, http.StatusBadRequest, operation, "InvalidPatchedSubmodel"), nil
	}

	err := error(nil)
	if patchIncludesSubmodelElements {
		err = s.submodelBackend.PatchSubmodel(ctx, decodedIdentifier, mergedSubmodel)
	} else {
		err = s.submodelBackend.PatchSubmodelMetadata(ctx, decodedIdentifier, mergedSubmodel)
	}
	if err != nil {
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		if common.IsErrNotFound(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}

		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "InternalServerError"), nil
	}

	return gen.Response(http.StatusNoContent, nil), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelByIDMetadata(ctx context.Context, submodelIdentifier string) (gen.ImplResponse, error) {
	const operation = "GetSubmodelByIDMetadata"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	sm, err := s.submodelBackend.GetSubmodelByID(ctx, string(decodedSubmodelIdentifier), "", true)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}

		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelByID"), nil
	}

	jsonSubmodel, err := jsonization.ToJsonable(sm)
	if err != nil {
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "ToJsonable"), nil
	}

	delete(jsonSubmodel, "submodelElements")

	return gen.Response(http.StatusOK, jsonSubmodel), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) PatchSubmodelByIDMetadata(ctx context.Context, submodelIdentifier string, submodelMetadata gen.SubmodelMetadata) (gen.ImplResponse, error) {
	const operation = "PatchSubmodelByIDMetadata"

	decodedIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	if submodelMetadata.ID != "" && decodedIdentifier != submodelMetadata.ID {
		return newAPIErrorResponse(errors.New("submodel ID in path and body do not match"), http.StatusBadRequest, operation, "IdMismatch"), nil
	}

	patchJSON, patchJSONErr := submodelMetadataToJSONPatch(submodelMetadata)
	if patchJSONErr != nil {
		return newAPIErrorResponse(patchJSONErr, http.StatusBadRequest, operation, "InvalidSubmodelMetadata"), nil
	}
	if rawPatchJSON, hasRawPatch := common.GetSubmodelMetadataPatch(ctx); hasRawPatch {
		patchJSON = rawPatchJSON
	}
	if patchJSON["modelType"] != "Submodel" {
		return newAPIErrorResponse(errors.New("modelType for Submodel metadata must be 'Submodel'"), http.StatusBadRequest, operation, "InvalidSubmodelMetadata"), nil
	}
	patchJSON["id"] = decodedIdentifier

	existingSubmodel, getErr := s.submodelBackend.GetSubmodelByID(ctx, decodedIdentifier, "core", true)
	if getErr != nil {
		if isNotFoundError(getErr) {
			return newAPIErrorResponse(getErr, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		return newAPIErrorResponse(getErr, http.StatusInternalServerError, operation, "GetSubmodelByID"), nil
	}

	existingJSON, existingJSONErr := jsonization.ToJsonable(existingSubmodel)
	if existingJSONErr != nil {
		return newAPIErrorResponse(existingJSONErr, http.StatusInternalServerError, operation, "ToJsonableCurrentSubmodel"), nil
	}

	mergedJSON := mergeJSONObjects(existingJSON, patchJSON)
	delete(mergedJSON, "submodelElements")
	mergedSubmodel, mergedErr := jsonization.SubmodelFromJsonable(mergedJSON)
	if mergedErr != nil {
		return newAPIErrorResponse(mergedErr, http.StatusBadRequest, operation, "InvalidPatchedSubmodel"), nil
	}

	if err := s.submodelBackend.PatchSubmodelMetadata(ctx, decodedIdentifier, mergedSubmodel); err != nil {
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrDenied(err) {
			return newAPIErrorResponse(err, http.StatusForbidden, operation, "Denied"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "PatchSubmodelMetadata"), nil
	}

	return gen.Response(http.StatusNoContent, nil), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelByIDValueOnly(ctx context.Context, submodelIdentifier string, level string, extent string) (gen.ImplResponse, error) {
	_ = extent
	const operation = "GetSubmodelByIDValueOnly"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	sm, err := s.submodelBackend.GetSubmodelByID(ctx, string(decodedSubmodelIdentifier), level, false)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelByID"), nil
	}

	valueOnly, convErr := gen.SubmodelToValueOnly(sm)
	if convErr != nil {
		return newAPIErrorResponse(convErr, http.StatusInternalServerError, operation, "SubmodelToValueOnly"), nil
	}

	return gen.Response(http.StatusOK, valueOnly), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) PatchSubmodelByIDValueOnly(ctx context.Context, submodelIdentifier string, body gen.SubmodelValue, level string) (gen.ImplResponse, error) {
	_ = level
	const operation = "PatchSubmodelByIDValueOnly"

	decodedIdentifier, err := common.DecodeString(submodelIdentifier)
	if err != nil {
		return newAPIErrorResponse(err, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	err = s.submodelBackend.UpdateSubmodelValueOnly(ctx, string(decodedIdentifier), body)
	if err != nil {
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		if common.IsErrNotFound(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, fmt.Sprintf("SubmodelOrSubmodelElementNotFound-%s", decodedIdentifier)), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "InternalServerError"), nil
	}
	return gen.Response(http.StatusNoContent, nil), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelByIDReference(ctx context.Context, submodelIdentifier string) (gen.ImplResponse, error) {
	const operation = "GetSubmodelByIDReference"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	reference, err := s.submodelBackend.GetSubmodelReference(ctx, decodedSubmodelIdentifier)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelReference"), nil
	}

	jsonableRef, convErr := jsonization.ToJsonable(reference)
	if convErr != nil {
		return newAPIErrorResponse(convErr, http.StatusInternalServerError, operation, "ToJsonable"), nil
	}

	return gen.Response(http.StatusOK, jsonableRef), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetSubmodelByIDPath(ctx context.Context, submodelIdentifier string, level string) (gen.ImplResponse, error) {
	const operation = "GetSubmodelByIDPath"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(submodelIdentifier)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	if !isLevelValid(level) {
		return invalidLevelResponse(operation), nil
	}

	paths, err := s.submodelBackend.GetSubmodelElementPaths(ctx, decodedSubmodelIdentifier, level)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElementPaths"), nil
	}

	return gen.Response(http.StatusOK, paths), nil
}

func (s *SubmodelRepositoryAPIAPIService) GetAllSubmodelRecentChanges(
	ctx context.Context,
	semanticID string,
	createdFrom time.Time,
	updatedFrom time.Time,
	limit int32,
	cursor string,
) (gen.ImplResponse, error) {
	const operation = "GetAllSubmodelRecentChanges"

	decodedCursor, decodeErr := decodeOptionalCursor(cursor)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadCursor"), nil
	}
	decodedSemanticID := ""
	if semanticID != "" {
		decodedSemanticID, decodeErr = common.DecodeString(semanticID)
		if decodeErr != nil {
			return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "BadSemanticID"), nil
		}
	}

	fetch := func(pageLimit int32, pageCursor string) ([]history.Row, string, error) {
		return s.submodelBackend.GetSubmodelRecentChanges(ctx, pageLimit, pageCursor, createdFrom, updatedFrom)
	}
	var rows []history.Row
	var nextCursor string
	var err error
	if semanticID == "" {
		rows, nextCursor, err = fetch(limit, decodedCursor)
	} else {
		rows, nextCursor, err = history.FilterRecentRows(limit, decodedCursor, fetch, func(row history.Row) (bool, error) {
			return matchesSubmodelRecentRowSemanticID(row, decodedSemanticID)
		})
	}
	if err != nil {
		if common.IsErrBadRequest(err) {
			return newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetRecentChanges"), nil
	}

	changes := make([]gen.SubmodelRecentChange, 0, len(rows))
	for _, row := range rows {
		if row.Deleted {
			changes = append(changes, gen.SubmodelRecentChange{
				RecentChange: gen.RecentChange{
					Type:      row.ChangeType,
					CreatedAt: row.CreatedAt,
					UpdatedAt: row.UpdatedAt,
				},
				Id: row.Identifier,
			})
			continue
		}

		submodel, fromJSONErr := jsonization.SubmodelFromJsonable(row.Snapshot)
		if fromJSONErr != nil {
			return newAPIErrorResponse(fromJSONErr, http.StatusInternalServerError, operation, "FromJsonable"), nil
		}
		semanticID, fromJSONErr := gen.JsonableReference(submodel.SemanticID())
		if fromJSONErr != nil {
			return newAPIErrorResponse(fromJSONErr, http.StatusInternalServerError, operation, "SemanticIdToJsonable"), nil
		}
		supplementalSemanticIDs, fromJSONErr := gen.JsonableReferences(submodel.SupplementalSemanticIDs())
		if fromJSONErr != nil {
			return newAPIErrorResponse(fromJSONErr, http.StatusInternalServerError, operation, "SupplementalSemanticIdsToJsonable"), nil
		}
		changes = append(changes, gen.SubmodelRecentChange{
			RecentChange: gen.RecentChange{
				Type:      row.ChangeType,
				CreatedAt: row.CreatedAt,
				UpdatedAt: row.UpdatedAt,
			},
			Id:                      submodel.ID(),
			SemanticId:              semanticID,
			SupplementalSemanticIds: supplementalSemanticIDs,
		})
	}

	return gen.Response(http.StatusOK, gen.GetAllSubmodelRecentChangesResult{
		PagingMetadata: gen.PagedResultPagingMetadata{Cursor: common.EncodeString(nextCursor)},
		Result:         changes,
	}), nil
}
func (s *SubmodelRepositoryAPIAPIService) GetSignedSubmodelByIDValueOnly(
	ctx context.Context,
	id string,
	_ /*level*/ string,
	_ /*extent*/ string,
) (gen.ImplResponse, error) {
	const operation = "GetSignedSubmodelByIDValueOnly"

	decodedSubmodelIdentifier, decodeErr := common.DecodeString(id)
	if decodeErr != nil {
		return newAPIErrorResponse(decodeErr, http.StatusBadRequest, operation, "MalformedSubmodelIdentifier"), nil
	}

	jwsString, err := s.submodelBackend.GetSignedSubmodelValueOnly(ctx, decodedSubmodelIdentifier)
	if err != nil {
		if isNotFoundError(err) {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelNotFound"), nil
		}
		if err.Error() == "JWS signing not configured: private key not loaded" {
			return newAPIErrorResponse(err, http.StatusNotFound, operation, "SigningNotConfigured"), nil
		}
		return newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSignedSubmodel"), nil
	}

	return gen.Response(http.StatusOK, jwsString), nil
}
