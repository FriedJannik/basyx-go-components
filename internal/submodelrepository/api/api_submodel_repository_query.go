package api

import (
	"context"
	"net/http"

	"github.com/FriedJannik/aas-go-sdk/jsonization"
	"github.com/eclipse-basyx/basyx-go-components/internal/common"
	gen "github.com/eclipse-basyx/basyx-go-components/internal/common/model"
	"github.com/eclipse-basyx/basyx-go-components/internal/common/model/grammar"
	auth "github.com/eclipse-basyx/basyx-go-components/internal/common/security"
	"golang.org/x/sync/errgroup"
)

// QuerySubmodels returns submodels matching the supplied query with pagination metadata.
func (s *SubmodelRepositoryAPIAPIService) QuerySubmodels(
	ctx context.Context,
	limit int32,
	cursor string,
	query grammar.Query,
) (gen.ImplResponse, error) {
	querySelectionCtx := auth.MergeQueryFilter(ctx, query)

	sms, nextCursor, err := s.submodelBackend.GetSubmodels(querySelectionCtx, limit, cursor, "")
	if err != nil {
		switch {
		case common.IsErrBadRequest(err):
			return common.NewErrorResponse(
				err, http.StatusBadRequest, "SMREPO", "QuerySubmodels", "BadRequest",
			), nil
		default:
			return common.NewErrorResponse(
				err, http.StatusInternalServerError, "SMREPO", "QuerySubmodels", "InternalServerError",
			), err
		}
	}

	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(8)

	for index := range sms {
		sm := sms[index]
		eg.Go(func() error {
			elementQueryCtx := auth.MergeQueryFilter(ctx, query)
			submodelElements, _, elementsErr := s.submodelBackend.GetSubmodelElements(elementQueryCtx, sm.ID(), nil, "", false, "")
			if elementsErr != nil {
				return elementsErr
			}

			sm.SetSubmodelElements(submodelElements)
			return nil
		})
	}

	if waitErr := eg.Wait(); waitErr != nil {
		if isNotFoundError(waitErr) {
			return common.NewErrorResponse(
				waitErr, http.StatusNotFound, "SMREPO", "QuerySubmodels", "SubmodelNotFound",
			), nil
		}
		return common.NewErrorResponse(
			waitErr, http.StatusInternalServerError, "SMREPO", "QuerySubmodels", "GetSubmodelElements",
		), waitErr
	}

	converted := make([]map[string]any, 0, len(sms))
	for _, sm := range sms {
		jsonable, convertErr := jsonization.ToJsonable(sm)
		if convertErr != nil {
			return common.NewErrorResponse(
				convertErr, http.StatusInternalServerError, "SMREPO", "QuerySubmodels", "InternalServerError",
			), convertErr
		}
		converted = append(converted, jsonable)
	}

	res := gen.GetSubmodelsResult{
		PagingMetadata: gen.PagedResultPagingMetadata{
			Cursor: nextCursor,
		},
		Result: converted,
	}

	return gen.Response(http.StatusOK, res), nil
}
