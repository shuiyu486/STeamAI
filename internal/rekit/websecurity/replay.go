package websecurity

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxAuthSecretBytes = 8 << 10

type AuthResolver func(string) (string, bool)

func PreflightReplay(
	requestBinding FileBinding,
	requestBytes,
	inventoryBytes []byte,
	resolveAuth AuthResolver,
) (ReplayRequest, Inventory, error) {
	request, inventory, err := decodeReplayInputs(requestBinding, requestBytes, inventoryBytes)
	if err != nil {
		return ReplayRequest{}, Inventory{}, err
	}
	if _, errorCode := resolveReplayAuth(request, inventory, resolveAuth); errorCode != "" {
		return ReplayRequest{}, Inventory{}, fmt.Errorf("bounded replay preflight failed: %s", errorCode)
	}
	return request, inventory, nil
}

func ExecuteReplay(
	ctx context.Context,
	requestBinding FileBinding,
	requestBytes,
	inventoryBytes []byte,
	resolveAuth AuthResolver,
) (ReplayResult, error) {
	request, inventory, err := decodeReplayInputs(requestBinding, requestBytes, inventoryBytes)
	if err != nil {
		return ReplayResult{}, err
	}

	result := ReplayResult{
		SchemaVersion: SchemaVersion,
		Kind:          ReplayResultKind,
		AdapterID:     ReplayAdapterID,
		Request:       requestBinding,
		Inventory:     request.Inventory,
		Target:        request.Target,
		Operation:     request.Operation,
		Limits:        request.Limits,
		Boundaries:    request.Boundaries,
	}
	authValue, errorCode := resolveReplayAuth(request, inventory, resolveAuth)
	if errorCode != "" {
		return failedBeforeDelivery(result, errorCode)
	}
	if err := ctx.Err(); err != nil {
		return failedBeforeDelivery(result, "context-unavailable")
	}

	targetURL, err := replayURL(request.Target, request.Operation)
	if err != nil {
		return ReplayResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, strings.ToUpper(request.Operation.Method), targetURL, nil)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("build bounded replay HTTP request: %w", err)
	}
	httpRequest.Header.Set("Accept", "*/*")
	httpRequest.Header.Set("User-Agent", "STeamAI-bounded-replay/1")
	if err := applyReplayAuth(httpRequest, request, inventory, authValue); err != nil {
		return ReplayResult{}, err
	}

	timeout := time.Duration(request.Limits.RuntimeMillis) * time.Millisecond
	transport := replayTransport(request.Target, timeout)
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		result.Status = "delivery-uncertain"
		result.Delivery = ReplayDelivery{Attempts: 1, Certain: false, ErrorCode: "delivery-uncertain"}
		if validateErr := ValidateReplayResult(result); validateErr != nil {
			return ReplayResult{}, validateErr
		}
		return result, nil
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, int64(request.Limits.ResponseBodyBytes)+1))
	if err != nil {
		return abortedAfterDelivery(result, "response-read")
	}
	if len(body) > request.Limits.ResponseBodyBytes {
		return abortedAfterDelivery(result, "response-body-limit")
	}
	actual := ActualResponse{
		StatusCode: response.StatusCode,
		Body:       DigestExpectation{SHA256: SHA256(body), Bytes: int64(len(body))},
		Headers:    responseHeaderDigests(response, request.Expected.Headers),
	}
	diff := compareResponse(request.Expected, actual)
	result.Status = "different"
	if diff.Match {
		result.Status = "matched"
	}
	result.Delivery = ReplayDelivery{Attempts: 1, Certain: true}
	result.Actual = &actual
	result.Diff = &diff
	if err := ValidateReplayResult(result); err != nil {
		return ReplayResult{}, err
	}
	return result, nil
}

func decodeReplayInputs(
	requestBinding FileBinding,
	requestBytes,
	inventoryBytes []byte,
) (ReplayRequest, Inventory, error) {
	if err := ValidateReplayRequestBinding(requestBinding); err != nil {
		return ReplayRequest{}, Inventory{}, err
	}
	if int64(len(requestBytes)) != requestBinding.Bytes || SHA256(requestBytes) != requestBinding.SHA256 {
		return ReplayRequest{}, Inventory{}, fmt.Errorf("bounded replay request binding does not match input bytes")
	}
	request, err := DecodeReplayRequest(requestBytes)
	if err != nil {
		return ReplayRequest{}, Inventory{}, err
	}
	if int64(len(inventoryBytes)) != request.Inventory.Bytes || SHA256(inventoryBytes) != request.Inventory.SHA256 {
		return ReplayRequest{}, Inventory{}, fmt.Errorf("bounded replay inventory binding does not match input bytes")
	}
	inventory, err := DecodeInventory(inventoryBytes)
	if err != nil {
		return ReplayRequest{}, Inventory{}, err
	}
	if err := validateReplayInventoryBinding(request, inventory); err != nil {
		return ReplayRequest{}, Inventory{}, err
	}
	return request, inventory, nil
}

func validateReplayInventoryBinding(request ReplayRequest, inventory Inventory) error {
	endpoint, ok := FindEndpoint(inventory, request.Operation.Key)
	if !ok || endpoint.Method != request.Operation.Method || endpoint.PathTemplate != request.Operation.PathTemplate {
		return fmt.Errorf("bounded replay operation drifted from the exact OpenAPI inventory")
	}
	if !concretePathMatchesTemplate(request.Operation.Path, endpoint.PathTemplate) {
		return fmt.Errorf("bounded replay concrete path drifted from the exact OpenAPI inventory")
	}
	serverMatch := false
	for _, server := range inventory.Servers {
		if server.Scheme == request.Target.Scheme && server.Host == request.Target.Host && server.Port == request.Target.Port && server.BasePath == request.Target.BasePath {
			serverMatch = true
			break
		}
	}
	if !serverMatch {
		return fmt.Errorf("bounded replay target is not an exact inventoried server")
	}
	if err := validateEndpointAuth(inventory, endpoint, request.Auth); err != nil {
		return err
	}
	return nil
}

func replayURL(target ReplayTarget, operation ReplayOperation) (string, error) {
	if err := validateReplayTarget(target); err != nil {
		return "", err
	}
	if err := validateReplayOperation(operation); err != nil {
		return "", err
	}
	requestPath := operation.Path
	if target.BasePath != "/" {
		requestPath = target.BasePath + operation.Path
	}
	if !validURLPath(requestPath, true) {
		return "", fmt.Errorf("bounded replay URL path is invalid")
	}
	host := net.JoinHostPort(target.Host, strconv.Itoa(target.Port))
	value := url.URL{Scheme: target.Scheme, Host: host, Path: requestPath}
	return value.String(), nil
}

func replayTransport(target ReplayTarget, timeout time.Duration) *http.Transport {
	expectedAddress := net.JoinHostPort(target.Host, strconv.Itoa(target.Port))
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: -1}
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" || address != expectedAddress {
				return nil, fmt.Errorf("bounded replay refused unexpected dial target")
			}
			return dialer.DialContext(ctx, network, expectedAddress)
		},
		ForceAttemptHTTP2:     false,
		DisableKeepAlives:     true,
		DisableCompression:    true,
		MaxIdleConns:          0,
		MaxIdleConnsPerHost:   0,
		MaxConnsPerHost:       1,
		IdleConnTimeout:       time.Second,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 0,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: target.Host,
		},
	}
}

func resolveReplayAuth(request ReplayRequest, inventory Inventory, resolve AuthResolver) (string, string) {
	if request.Auth == nil {
		return "", ""
	}
	if resolve == nil {
		return "", "auth-ref-unavailable"
	}
	value, ok := resolve(request.Auth.AuthRef)
	if !ok || value == "" {
		return "", "auth-ref-unavailable"
	}
	if len(value) > maxAuthSecretBytes || strings.ContainsAny(value, "\r\n\x00") {
		return "", "auth-value-invalid"
	}
	if _, ok := FindAuthScheme(inventory, request.Auth.Scheme); !ok {
		return "", "auth-scheme-drift"
	}
	return value, ""
}

func applyReplayAuth(httpRequest *http.Request, request ReplayRequest, inventory Inventory, value string) error {
	if request.Auth == nil {
		return nil
	}
	scheme, ok := FindAuthScheme(inventory, request.Auth.Scheme)
	if !ok || !scheme.ReplaySupported {
		return fmt.Errorf("bounded replay auth scheme is absent or unsupported")
	}
	switch scheme.Type {
	case "http":
		if scheme.Scheme != "bearer" {
			return fmt.Errorf("bounded replay supports only bearer HTTP auth")
		}
		httpRequest.Header.Set("Authorization", "Bearer "+value)
	case "apiKey":
		if scheme.In != "header" {
			return fmt.Errorf("bounded replay supports only header apiKey auth")
		}
		httpRequest.Header.Set(scheme.Parameter, value)
	default:
		return fmt.Errorf("bounded replay auth scheme type is unsupported")
	}
	return nil
}

func responseHeaderDigests(response *http.Response, expected []HeaderExpectation) []HeaderExpectation {
	headers := make([]HeaderExpectation, 0, len(expected))
	for _, item := range expected {
		value := []byte(strings.Join(response.Header.Values(http.CanonicalHeaderKey(item.Name)), ","))
		headers = append(headers, HeaderExpectation{Name: item.Name, SHA256: SHA256(value), Bytes: int64(len(value))})
	}
	return headers
}

func compareResponse(expected ExpectedResponse, actual ActualResponse) ReplayDiff {
	statusMatch := expected.StatusCode == actual.StatusCode
	bodyMatch := expected.Body == actual.Body
	headersMatch := len(expected.Headers) == len(actual.Headers)
	if headersMatch {
		for index := range expected.Headers {
			if expected.Headers[index] != actual.Headers[index] {
				headersMatch = false
				break
			}
		}
	}
	return ReplayDiff{
		Match:        statusMatch && bodyMatch && headersMatch,
		StatusMatch:  statusMatch,
		BodyMatch:    bodyMatch,
		HeadersMatch: headersMatch,
	}
}

func failedBeforeDelivery(result ReplayResult, errorCode string) (ReplayResult, error) {
	result.Status = "failed-before-delivery"
	result.Delivery = ReplayDelivery{Attempts: 0, Certain: true, ErrorCode: errorCode}
	if err := ValidateReplayResult(result); err != nil {
		return ReplayResult{}, err
	}
	return result, nil
}

func abortedAfterDelivery(result ReplayResult, errorCode string) (ReplayResult, error) {
	result.Status = "aborted-after-delivery"
	result.Delivery = ReplayDelivery{Attempts: 1, Certain: true, ErrorCode: errorCode}
	if err := ValidateReplayResult(result); err != nil {
		return ReplayResult{}, err
	}
	return result, nil
}
