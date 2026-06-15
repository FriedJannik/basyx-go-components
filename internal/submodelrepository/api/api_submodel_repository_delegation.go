package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FriedJannik/aas-go-sdk/jsonization"
	"github.com/FriedJannik/aas-go-sdk/types"
	"github.com/eclipse-basyx/basyx-go-components/internal/common"
	"github.com/eclipse-basyx/basyx-go-components/internal/common/asyncbulk"
	gen "github.com/eclipse-basyx/basyx-go-components/internal/common/model"
	auth "github.com/eclipse-basyx/basyx-go-components/internal/common/security"
	persistencepostgresql "github.com/eclipse-basyx/basyx-go-components/internal/submodelrepository/persistence"
	openapi "github.com/eclipse-basyx/basyx-go-components/pkg/submodelrepositoryapi"
)

func parseDelegationTimeout(clientTimeoutDuration string) (time.Duration, error) {
	if strings.TrimSpace(clientTimeoutDuration) == "" {
		return defaultDelegationTimeout, nil
	}

	trimmed := strings.TrimSpace(clientTimeoutDuration)
	if !strings.HasPrefix(trimmed, "P") && !strings.HasPrefix(trimmed, "-P") {
		return 0, fmt.Errorf("SMREPO-PARSETO-INVALID clientTimeoutDuration '%s' is not an ISO8601 duration", clientTimeoutDuration)
	}

	sign := 1.0
	if strings.HasPrefix(trimmed, "-P") {
		sign = -1.0
		trimmed = strings.TrimPrefix(trimmed, "-")
	}

	trimmed = strings.TrimPrefix(trimmed, "P")
	parts := strings.SplitN(trimmed, "T", 2)
	datePart := parts[0]
	timePart := ""
	if len(parts) == 2 {
		timePart = parts[1]
	}

	if strings.Contains(datePart, "Y") || strings.Contains(datePart, "M") {
		return 0, fmt.Errorf("SMREPO-PARSETO-UNSUPPORTED years and months are not supported in clientTimeoutDuration")
	}

	remainingDate := datePart
	var totalDuration time.Duration
	if strings.Contains(remainingDate, "D") {
		dayParts := strings.SplitN(remainingDate, "D", 2)
		days, err := strconv.Atoi(dayParts[0])
		if err != nil {
			return 0, fmt.Errorf("SMREPO-PARSETO-PARSEDAYS %w", err)
		}
		totalDuration += time.Duration(days) * 24 * time.Hour
		remainingDate = dayParts[1]
	}

	if remainingDate != "" {
		return 0, fmt.Errorf("SMREPO-PARSETO-INVALIDDATE unsupported date part '%s'", remainingDate)
	}

	remainingTime := timePart
	if strings.Contains(remainingTime, "H") {
		hourParts := strings.SplitN(remainingTime, "H", 2)
		hours, err := strconv.Atoi(hourParts[0])
		if err != nil {
			return 0, fmt.Errorf("SMREPO-PARSETO-PARSEHOURS %w", err)
		}
		totalDuration += time.Duration(hours) * time.Hour
		remainingTime = hourParts[1]
	}

	if strings.Contains(remainingTime, "M") {
		minuteParts := strings.SplitN(remainingTime, "M", 2)
		minutes, err := strconv.Atoi(minuteParts[0])
		if err != nil {
			return 0, fmt.Errorf("SMREPO-PARSETO-PARSEMINUTES %w", err)
		}
		totalDuration += time.Duration(minutes) * time.Minute
		remainingTime = minuteParts[1]
	}

	if strings.Contains(remainingTime, "S") {
		secondsParts := strings.SplitN(remainingTime, "S", 2)
		seconds, err := strconv.ParseFloat(secondsParts[0], 64)
		if err != nil {
			return 0, fmt.Errorf("SMREPO-PARSETO-PARSESECONDS %w", err)
		}
		totalDuration += time.Duration(seconds * float64(time.Second))
		remainingTime = secondsParts[1]
	}

	if strings.TrimSpace(remainingTime) != "" {
		return 0, fmt.Errorf("SMREPO-PARSETO-INVALIDTIME unsupported time part '%s'", remainingTime)
	}

	computedDuration := time.Duration(float64(totalDuration) * sign)
	if computedDuration <= 0 {
		return 0, errors.New("SMREPO-PARSETO-NONPOSITIVE clientTimeoutDuration must resolve to a positive duration")
	}

	return computedDuration, nil
}

func resolveDelegationURL(element types.ISubmodelElement) (string, error) {
	if element == nil {
		return "", errors.New("SMREPO-RSLVDEL-NILELEMENT submodel element is nil")
	}

	if element.ModelType() != types.ModelTypeOperation {
		return "", common.NewErrBadRequest("invoke is only valid for Operation submodel elements")
	}

	for _, qualifier := range element.Qualifiers() {
		if qualifier == nil {
			continue
		}
		if qualifier.Type() == invocationDelegationQualifierType && qualifier.Value() != nil {
			delegationTarget := strings.TrimSpace(*qualifier.Value())
			if delegationTarget == "" {
				return "", errors.New("SMREPO-RSLVDEL-EMPTYURL invocationDelegation qualifier value is empty")
			}
			return delegationTarget, nil
		}
	}

	return "", errors.New("SMREPO-RSLVDEL-MISSINGQUAL invocationDelegation qualifier not found on operation")
}

func buildDelegatedOperationInput(operationRequest gen.OperationRequest) []types.IOperationVariable {
	delegatedInput := make([]types.IOperationVariable, 0, len(operationRequest.InputArguments)+len(operationRequest.InoutputArguments))
	delegatedInput = append(delegatedInput, operationRequest.InputArguments...)
	delegatedInput = append(delegatedInput, operationRequest.InoutputArguments...)
	return delegatedInput
}

func serializeDelegatedOperationPayload(payload []types.IOperationVariable) ([]byte, error) {
	jsonablePayload := make([]any, 0, len(payload))
	for index, operationVariable := range payload {
		jsonableValue, err := jsonization.ToJsonable(operationVariable)
		if err != nil {
			return nil, fmt.Errorf("SMREPO-SERDEL-TOJSONABLE-%d %w", index, err)
		}
		jsonablePayload = append(jsonablePayload, jsonableValue)
	}

	requestBody, err := json.Marshal(jsonablePayload)
	if err != nil {
		return nil, fmt.Errorf("SMREPO-SERDEL-MARSHAL %w", err)
	}

	return requestBody, nil
}

func toDelegatedOperationResultPayload(outputArguments []types.IOperationVariable, inoutputArguments []types.IOperationVariable) map[string]any {
	return map[string]any{
		"executionState":    "Completed",
		"success":           true,
		"outputArguments":   outputArguments,
		"inoutputArguments": inoutputArguments,
	}
}

func toOperationVariables(payload any) ([]types.IOperationVariable, bool) {
	if payload == nil {
		return nil, false
	}

	if alreadyTyped, ok := payload.([]types.IOperationVariable); ok {
		return alreadyTyped, true
	}

	switch typedPayload := payload.(type) {
	case []any:
		operationVariables := make([]types.IOperationVariable, 0, len(typedPayload))
		for _, item := range typedPayload {
			operationVariable, err := jsonization.OperationVariableFromJsonable(item)
			if err != nil {
				return nil, false
			}
			operationVariables = append(operationVariables, operationVariable)
		}
		return operationVariables, true
	case []map[string]any:
		operationVariables := make([]types.IOperationVariable, 0, len(typedPayload))
		for _, item := range typedPayload {
			operationVariable, err := jsonization.OperationVariableFromJsonable(item)
			if err != nil {
				return nil, false
			}
			operationVariables = append(operationVariables, operationVariable)
		}
		return operationVariables, true
	case map[string]any:
		if _, hasValue := typedPayload["value"]; !hasValue {
			return nil, false
		}
		operationVariable, err := jsonization.OperationVariableFromJsonable(typedPayload)
		if err != nil {
			return nil, false
		}
		return []types.IOperationVariable{operationVariable}, true
	default:
		return nil, false
	}
}

func toDelegatedOperationResultPayloadFromBody(delegatedBody any) (map[string]any, bool) {
	if delegatedOutput, ok := delegatedBody.([]types.IOperationVariable); ok {
		return toDelegatedOperationResultPayload(delegatedOutput, []types.IOperationVariable{}), true
	}

	delegatedBodyMap, ok := delegatedBody.(map[string]any)
	if !ok {
		return nil, false
	}

	outputArguments, outputOK := toOperationVariables(delegatedBodyMap["outputArguments"])
	inoutputArguments, inoutputOK := toOperationVariables(delegatedBodyMap["inoutputArguments"])
	if !outputOK && !inoutputOK {
		return nil, false
	}

	if !outputOK {
		outputArguments = []types.IOperationVariable{}
	}
	if !inoutputOK {
		inoutputArguments = []types.IOperationVariable{}
	}

	return toDelegatedOperationResultPayload(outputArguments, inoutputArguments), true
}

func parseDelegationAsyncTTL() time.Duration {
	rawTTL := strings.TrimSpace(os.Getenv(delegationAsyncTTLKey))
	if rawTTL == "" {
		return defaultDelegationAsyncTTL
	}

	parsedTTL, err := time.ParseDuration(rawTTL)
	if err != nil || parsedTTL <= 0 {
		return defaultDelegationAsyncTTL
	}

	return parsedTTL
}

func parseTrustedDelegationHosts() map[string]struct{} {
	rawHosts := strings.TrimSpace(os.Getenv(delegationTrustedHostsKey))
	if rawHosts == "" {
		return map[string]struct{}{}
	}

	trustedHosts := map[string]struct{}{}
	for _, rawHost := range strings.Split(rawHosts, ",") {
		host := strings.ToLower(strings.TrimSpace(rawHost))
		if host == "" {
			continue
		}
		trustedHosts[host] = struct{}{}
	}

	return trustedHosts
}

func isTrustedDelegationHost(host string) bool {
	normalizedHost := strings.ToLower(strings.TrimSpace(host))
	if normalizedHost == "" {
		return false
	}

	if normalizedHost == "localhost" || strings.HasSuffix(normalizedHost, ".localhost") {
		return true
	}

	if ip, parseErr := netip.ParseAddr(normalizedHost); parseErr == nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}

	if parsedIP := net.ParseIP(normalizedHost); parsedIP != nil {
		return parsedIP.IsLoopback() || parsedIP.IsPrivate() || parsedIP.IsLinkLocalUnicast() || parsedIP.IsLinkLocalMulticast()
	}

	if strings.HasSuffix(normalizedHost, ".internal") || strings.HasSuffix(normalizedHost, ".svc") || strings.HasSuffix(normalizedHost, ".cluster.local") {
		return true
	}

	trustedHosts := parseTrustedDelegationHosts()
	_, trusted := trustedHosts[normalizedHost]
	return trusted
}

func shouldForwardAuthorizationHeader(parsedDelegationURL *url.URL) bool {
	if parsedDelegationURL == nil {
		return false
	}

	return isTrustedDelegationHost(parsedDelegationURL.Hostname())
}

func isSuccessfulHTTPStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

func doDelegatedOperationCall(ctx context.Context, delegationURL string, payload []types.IOperationVariable, timeout time.Duration) (int, any, error) {
	parsedDelegationURL, parseErr := url.Parse(delegationURL)
	if parseErr != nil {
		return 0, nil, fmt.Errorf("SMREPO-DOOPDELG-PARSEURL %w", parseErr)
	}
	if parsedDelegationURL.Scheme != "http" && parsedDelegationURL.Scheme != "https" {
		return 0, nil, errors.New("SMREPO-DOOPDELG-UNSUPPORTEDSCHEME delegation URL must use http or https")
	}
	if strings.TrimSpace(parsedDelegationURL.Host) == "" {
		return 0, nil, errors.New("SMREPO-DOOPDELG-MISSINGHOST delegation URL host is missing")
	}
	if !isTrustedDelegationHost(parsedDelegationURL.Hostname()) {
		return 0, nil, fmt.Errorf("SMREPO-DOOPDELG-UNTRUSTEDHOST delegation URL host %q is not in SMREPO_DELEGATION_TRUSTED_HOSTS allowlist", parsedDelegationURL.Host)
	}

	requestBody, marshalErr := serializeDelegatedOperationPayload(payload)
	if marshalErr != nil {
		return 0, nil, fmt.Errorf("SMREPO-DOOPDELG-MARSHALREQ %w", marshalErr)
	}

	request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, delegationURL, bytes.NewReader(requestBody))
	if requestErr != nil {
		return 0, nil, fmt.Errorf("SMREPO-DOOPDELG-CREATEREQ %w", requestErr)
	}

	request.Header.Set("Content-Type", "application/json")
	authorizationHeader := common.AuthorizationHeaderFromContext(ctx)
	if strings.TrimSpace(authorizationHeader) != "" && shouldForwardAuthorizationHeader(parsedDelegationURL) {
		request.Header.Set("Authorization", authorizationHeader)
	}

	httpClient := &http.Client{Timeout: timeout}
	// #nosec G704 -- delegation target is validated for scheme and host before request execution.
	response, responseErr := httpClient.Do(request)
	if responseErr != nil {
		return 0, nil, fmt.Errorf("SMREPO-DOOPDELG-EXECREQ %w", responseErr)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBytes, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return 0, nil, fmt.Errorf("SMREPO-DOOPDELG-READRESP %w", readErr)
	}

	if len(responseBytes) == 0 {
		return response.StatusCode, []types.IOperationVariable{}, nil
	}

	var delegatedOutput []types.IOperationVariable
	if unmarshalErr := json.Unmarshal(responseBytes, &delegatedOutput); unmarshalErr == nil {
		return response.StatusCode, delegatedOutput, nil
	}

	var passthroughBody any
	if unmarshalErr := json.Unmarshal(responseBytes, &passthroughBody); unmarshalErr != nil {
		passthroughBody = map[string]any{"message": string(responseBytes)}
	}

	return response.StatusCode, passthroughBody, nil
}

func loadOperationElement(ctx context.Context, backend persistencepostgresql.SubmodelDatabase, decodedSubmodelIdentifier string, idShortPath string, operation string) (types.ISubmodelElement, gen.ImplResponse, bool) {
	element, err := backend.GetSubmodelElement(ctx, decodedSubmodelIdentifier, idShortPath, false, "")
	if err != nil {
		if isNotFoundError(err) {
			return nil, newAPIErrorResponse(err, http.StatusNotFound, operation, "SubmodelElementNotFound"), false
		}
		if common.IsErrBadRequest(err) {
			return nil, newAPIErrorResponse(err, http.StatusBadRequest, operation, "BadRequest"), false
		}
		return nil, newAPIErrorResponse(err, http.StatusInternalServerError, operation, "GetSubmodelElement"), false
	}

	return element, gen.ImplResponse{}, true
}

// InvokeOperationValueOnly - Synchronously or asynchronously invokes an Operation at a specified path

// InvokeOperationAsync - Asynchronously invokes an Operation at a specified path

func asyncRecordMatchesOperation(record asyncbulk.Record, decodedSubmodelIdentifier string, idShortPath string) bool {
	return record.Metadata[delegatedAsyncSubmodelIdentifierMetadataKey] == decodedSubmodelIdentifier &&
		record.Metadata[delegatedAsyncIDShortPathMetadataKey] == idShortPath
}

// GetOperationAsyncStatus - Returns the status of an asynchronously invoked Operation

// GetOperationAsyncResult - Returns the Operation result of an asynchronously invoked Operation

// GetOperationAsyncResultValueOnly - Returns the Operation result of an asynchronously invoked Operation

// QuerySubmodels returns all Submodels that match the input query.
// It supports filtering based on the query language and provides pagination through cursor-based navigation.
//
// Parameters:
//   - ctx: Request context for security and cancellation
//   - limit: Maximum number of submodels to return
//   - cursor: Pagination cursor for continuing from previous results
//   - query: Query object containing the filter condition
//
// Returns:
//   - gen.ImplResponse: Response containing paginated submodel results
//   - error: Error if the operation fails

// InvokeOperationSubmodelRepo invokes an Operation submodel element synchronously or asynchronously.
func (s *SubmodelRepositoryAPIAPIService) InvokeOperationSubmodelRepo(ctx context.Context, submodelIdentifier string, idShortPath string, operationRequest gen.OperationRequest, async bool) (gen.ImplResponse, error) {
	const operation = "InvokeOperationSubmodelRepo"

	if async {
		return s.InvokeOperationAsync(ctx, submodelIdentifier, idShortPath, operationRequest)
	}

	decodedSubmodelIdentifier, response, ok := decodeSubmodelIdentifierOrAPIError(submodelIdentifier, operation)
	if !ok {
		return response, nil
	}

	element, response, ok := loadOperationElement(ctx, s.submodelBackend, decodedSubmodelIdentifier, idShortPath, operation)
	if !ok {
		return response, nil
	}

	delegationURL, delegationErr := resolveDelegationURL(element)
	if delegationErr != nil {
		if common.IsErrBadRequest(delegationErr) {
			return newAPIErrorResponse(delegationErr, http.StatusMethodNotAllowed, operation, "InvokeOnlyValidForOperation"), nil
		}
		return newAPIErrorResponse(delegationErr, http.StatusNotImplemented, operation, "OperationDelegationMissing"), nil
	}

	timeout, timeoutErr := parseDelegationTimeout(operationRequest.ClientTimeoutDuration)
	if timeoutErr != nil {
		return newAPIErrorResponse(timeoutErr, http.StatusBadRequest, operation, "InvalidClientTimeoutDuration"), nil
	}

	statusCode, delegatedBody, delegateErr := doDelegatedOperationCall(ctx, delegationURL, buildDelegatedOperationInput(operationRequest), timeout)
	if delegateErr != nil {
		return newAPIErrorResponse(delegateErr, http.StatusInternalServerError, operation, "DelegateOperationCall"), nil
	}

	if !isSuccessfulHTTPStatus(statusCode) {
		return gen.Response(statusCode, delegatedBody), nil
	}

	if resultPayload, ok := toDelegatedOperationResultPayloadFromBody(delegatedBody); ok {
		return gen.Response(http.StatusOK, resultPayload), nil
	}

	return gen.Response(http.StatusOK, delegatedBody), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) InvokeOperationValueOnly(ctx context.Context, aasIdentifier string, submodelIdentifier string, idShortPath string, operationRequestValueOnly gen.OperationRequestValueOnly, async bool) (gen.ImplResponse, error) {
	_ = ctx
	_ = aasIdentifier
	_ = submodelIdentifier
	_ = idShortPath
	_ = operationRequestValueOnly
	_ = async

	delegationUnsupportedErr := errors.New("SMREPO-INVOPVAL-DELEGUNSUPPORTED value-only delegation is not supported")
	return newAPIErrorResponse(delegationUnsupportedErr, http.StatusBadRequest, "InvokeOperationValueOnly", "DelegationValueOnlyNotSupported"), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) InvokeOperationAsync(ctx context.Context, submodelIdentifier string, idShortPath string, operationRequest gen.OperationRequest) (gen.ImplResponse, error) {
	const operation = "InvokeOperationAsync"

	decodedSubmodelIdentifier, response, ok := decodeSubmodelIdentifierOrAPIError(submodelIdentifier, operation)
	if !ok {
		return response, nil
	}

	element, response, ok := loadOperationElement(ctx, s.submodelBackend, decodedSubmodelIdentifier, idShortPath, operation)
	if !ok {
		return response, nil
	}

	delegationURL, delegationErr := resolveDelegationURL(element)
	if delegationErr != nil {
		if common.IsErrBadRequest(delegationErr) {
			return newAPIErrorResponse(delegationErr, http.StatusMethodNotAllowed, operation, "InvokeOnlyValidForOperation"), nil
		}
		return newAPIErrorResponse(delegationErr, http.StatusNotImplemented, operation, "OperationDelegationMissing"), nil
	}

	timeout, timeoutErr := parseDelegationTimeout(operationRequest.ClientTimeoutDuration)
	if timeoutErr != nil {
		return newAPIErrorResponse(timeoutErr, http.StatusBadRequest, operation, "InvalidClientTimeoutDuration"), nil
	}

	handleID, handleErr := s.asyncManager.Start(auth.OwnerKeyFromContext(ctx))
	if handleErr != nil {
		return newAPIErrorResponse(handleErr, http.StatusInternalServerError, operation, "CreateAsyncHandle"), nil
	}
	s.asyncManager.Update(handleID, func(record asyncbulk.Record) asyncbulk.Record {
		record.Metadata = map[string]string{
			delegatedAsyncSubmodelIdentifierMetadataKey: decodedSubmodelIdentifier,
			delegatedAsyncIDShortPathMetadataKey:        idShortPath,
		}
		return record
	})

	go func() {
		delegationCtx := context.WithoutCancel(ctx)

		statusCode, delegatedBody, delegateErr := doDelegatedOperationCall(delegationCtx, delegationURL, buildDelegatedOperationInput(operationRequest), timeout)
		if delegateErr != nil {
			s.asyncManager.Update(handleID, func(record asyncbulk.Record) asyncbulk.Record {
				record.ExecutionState = "Failed"
				record.ErrorStatus = http.StatusInternalServerError
				record.ErrorBody = map[string]any{"message": delegateErr.Error()}
				return record
			})
			return
		}

		if !isSuccessfulHTTPStatus(statusCode) {
			s.asyncManager.Update(handleID, func(record asyncbulk.Record) asyncbulk.Record {
				record.ExecutionState = "Failed"
				record.ErrorStatus = statusCode
				record.ErrorBody = delegatedBody
				return record
			})
			return
		}

		finalResult := delegatedBody
		if resultPayload, resultOK := toDelegatedOperationResultPayloadFromBody(delegatedBody); resultOK {
			finalResult = resultPayload
		}

		s.asyncManager.Update(handleID, func(record asyncbulk.Record) asyncbulk.Record {
			record.ExecutionState = "Completed"
			record.Payload = finalResult
			record.ErrorStatus = 0
			record.ErrorBody = nil
			return record
		})
	}()

	return gen.Response(http.StatusAccepted, map[string]any{"handleId": handleID}), nil
}

// InvokeOperationAsyncValueOnly - Asynchronously invokes an Operation at a specified path
//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) InvokeOperationAsyncValueOnly(ctx context.Context, aasIdentifier string, submodelIdentifier string, idShortPath string, operationRequestValueOnly gen.OperationRequestValueOnly) (gen.ImplResponse, error) {
	_ = ctx
	_ = aasIdentifier
	_ = submodelIdentifier
	_ = idShortPath
	_ = operationRequestValueOnly

	delegationUnsupportedErr := errors.New("SMREPO-INVOPASYVAL-DELEGUNSUPPORTED value-only delegation is not supported")
	return newAPIErrorResponse(delegationUnsupportedErr, http.StatusBadRequest, "InvokeOperationAsyncValueOnly", "DelegationValueOnlyNotSupported"), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetOperationAsyncStatus(ctx context.Context, submodelIdentifier string, idShortPath string, handleID string) (gen.ImplResponse, error) {
	_ = ctx
	const operation = "GetOperationAsyncStatus"

	decodedSubmodelIdentifier, response, ok := decodeSubmodelIdentifierOrAPIError(submodelIdentifier, operation)
	if !ok {
		return response, nil
	}

	record, found := s.asyncManager.GetForOwner(handleID, auth.OwnerKeyFromContext(ctx))
	if !found || !asyncRecordMatchesOperation(record, decodedSubmodelIdentifier, idShortPath) {
		handleErr := common.NewErrNotFound(handleID)
		return newAPIErrorResponse(handleErr, http.StatusNotFound, operation, "HandleNotFound"), nil
	}

	if record.ExecutionState == "Running" {
		return gen.Response(http.StatusOK, map[string]any{"executionState": "Running", "success": true}), nil
	}

	location := fmt.Sprintf(
		"/submodels/%s/submodel-elements/%s/operation-results/%s",
		url.PathEscape(submodelIdentifier),
		url.PathEscape(idShortPath),
		url.PathEscape(handleID),
	)

	return gen.Response(http.StatusFound, openapi.Redirect{Location: location}), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetOperationAsyncResult(ctx context.Context, submodelIdentifier string, idShortPath string, handleID string) (gen.ImplResponse, error) {
	_ = ctx
	const operation = "GetOperationAsyncResult"

	decodedSubmodelIdentifier, response, ok := decodeSubmodelIdentifierOrAPIError(submodelIdentifier, operation)
	if !ok {
		return response, nil
	}

	record, found := s.asyncManager.GetForOwner(handleID, auth.OwnerKeyFromContext(ctx))
	if !found || !asyncRecordMatchesOperation(record, decodedSubmodelIdentifier, idShortPath) {
		handleErr := common.NewErrNotFound(handleID)
		return newAPIErrorResponse(handleErr, http.StatusNotFound, operation, "HandleNotFound"), nil
	}

	if record.ExecutionState == "Running" {
		notCompletedErr := errors.New("SMREPO-GETOPASYRES-RUNNING operation is still running")
		return newAPIErrorResponse(notCompletedErr, http.StatusBadRequest, operation, "OperationStillRunning"), nil
	}

	if record.ExecutionState == "Failed" {
		if record.ErrorStatus <= 0 {
			return newAPIErrorResponse(errors.New("SMREPO-GETOPASYRES-FAILED delegated operation failed"), http.StatusInternalServerError, operation, "DelegatedOperationFailed"), nil
		}
		return gen.Response(record.ErrorStatus, record.ErrorBody), nil
	}

	return gen.Response(http.StatusOK, record.Payload), nil
}

//
//nolint:revive
func (s *SubmodelRepositoryAPIAPIService) GetOperationAsyncResultValueOnly(ctx context.Context, submodelIdentifier string, idShortPath string, handleID string) (gen.ImplResponse, error) {
	_ = ctx
	_ = submodelIdentifier
	_ = idShortPath
	_ = handleID

	delegationUnsupportedErr := errors.New("SMREPO-GETOPASYRESVAL-DELEGUNSUPPORTED value-only async result for delegated operation is not supported")
	return newAPIErrorResponse(delegationUnsupportedErr, http.StatusNotImplemented, "GetOperationAsyncResultValueOnly", "DelegationValueOnlyNotSupported"), nil
}
