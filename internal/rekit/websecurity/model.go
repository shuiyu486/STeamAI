package websecurity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	SchemaVersion = 1

	InventoryKind      = "web-security-openapi-inventory"
	InventoryAdapterID = "openapi-v3-json-inventory"
	ReplayRequestKind  = "web-security-bounded-replay-request"
	ReplayResultKind   = "web-security-bounded-replay-result"
	ReplayAdapterID    = "bounded-http-replay"
	ReplayRequestRoot  = "inputs/bounded-replay-requests"

	MaxOpenAPIBytes       = 4 << 20
	MaxInventoryBytes     = 1 << 20
	MaxReplayRequestBytes = 128 << 10
	MaxReplayResultBytes  = 128 << 10
	MaxEndpoints          = 2048
	MaxAuthSchemes        = 128
	MaxServers            = 32
	MaxParameters         = 256
	MaxMediaTypes         = 32
	MaxStringBytes        = 512
	MaxResponseBodyBytes  = 1 << 20
	MaxRuntimeMillis      = 30_000
)

var (
	sha256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	authRefPattern = regexp.MustCompile(`^STEAMAI_AUTH_[A-Z0-9_]{1,96}$`)
	hostPattern    = regexp.MustCompile(`^[a-z0-9.-]+$`)
	methodOrder    = map[string]int{"get": 0, "head": 1, "options": 2, "post": 3, "put": 4, "patch": 5, "delete": 6, "trace": 7}
)

type FileBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type InventoryBoundaries struct {
	ReadOnlyInput        bool `json:"readOnlyInput"`
	NoNetwork            bool `json:"noNetwork"`
	NoSecretsPersisted   bool `json:"noSecretsPersisted"`
	NoCatalogEntryExec   bool `json:"noCatalogEntryExecution"`
	NoAuthorityConfirmed bool `json:"noAuthorityOrConfirmed"`
}

type Server struct {
	Scheme   string `json:"scheme"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	BasePath string `json:"basePath"`
}

type AuthScheme struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	In              string `json:"in,omitempty"`
	Parameter       string `json:"parameter,omitempty"`
	Scheme          string `json:"scheme,omitempty"`
	ReplaySupported bool   `json:"replaySupported"`
}

type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
}

type Endpoint struct {
	Key                 string      `json:"key"`
	OperationID         string      `json:"operationId,omitempty"`
	Method              string      `json:"method"`
	PathTemplate        string      `json:"pathTemplate"`
	Parameters          []Parameter `json:"parameters"`
	RequestBodyRequired bool        `json:"requestBodyRequired"`
	RequestMediaTypes   []string    `json:"requestMediaTypes"`
	Security            [][]string  `json:"security"`
}

type Inventory struct {
	SchemaVersion  int                 `json:"schemaVersion"`
	Kind           string              `json:"kind"`
	AdapterID      string              `json:"adapterId"`
	OpenAPIVersion string              `json:"openapiVersion"`
	Source         FileBinding         `json:"source"`
	Servers        []Server            `json:"servers"`
	AuthSchemes    []AuthScheme        `json:"authSchemes"`
	Endpoints      []Endpoint          `json:"endpoints"`
	Warnings       []string            `json:"warnings"`
	Boundaries     InventoryBoundaries `json:"boundaries"`
}

type ReplayTarget struct {
	Scheme   string `json:"scheme"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	BasePath string `json:"basePath"`
}

type ReplayOperation struct {
	Key          string `json:"key"`
	Method       string `json:"method"`
	PathTemplate string `json:"pathTemplate"`
	Path         string `json:"path"`
}

type ReplayAuth struct {
	Scheme  string `json:"scheme"`
	AuthRef string `json:"authRef"`
}

type DigestExpectation struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type HeaderExpectation struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ExpectedResponse struct {
	StatusCode int                 `json:"statusCode"`
	Body       DigestExpectation   `json:"body"`
	Headers    []HeaderExpectation `json:"headers"`
}

type ReplayLimits struct {
	RuntimeMillis     int `json:"runtimeMillis"`
	ResponseBodyBytes int `json:"responseBodyBytes"`
	Requests          int `json:"requests"`
	Redirects         int `json:"redirects"`
}

type ReplayBoundaries struct {
	LoopbackOnly         bool `json:"loopbackOnly"`
	NoAmbientProxy       bool `json:"noAmbientProxy"`
	NoRedirects          bool `json:"noRedirects"`
	NoRetries            bool `json:"noRetries"`
	NoRequestBody        bool `json:"noRequestBody"`
	NoSecretsPersisted   bool `json:"noSecretsPersisted"`
	NoCatalogEntryExec   bool `json:"noCatalogEntryExecution"`
	NoAuthorityConfirmed bool `json:"noAuthorityOrConfirmed"`
}

type ReplayRequest struct {
	SchemaVersion int              `json:"schemaVersion"`
	Kind          string           `json:"kind"`
	AdapterID     string           `json:"adapterId"`
	Inventory     FileBinding      `json:"inventory"`
	Target        ReplayTarget     `json:"target"`
	Operation     ReplayOperation  `json:"operation"`
	Auth          *ReplayAuth      `json:"auth,omitempty"`
	Expected      ExpectedResponse `json:"expected"`
	Limits        ReplayLimits     `json:"limits"`
	Boundaries    ReplayBoundaries `json:"boundaries"`
}

type ActualResponse struct {
	StatusCode int                 `json:"statusCode"`
	Body       DigestExpectation   `json:"body"`
	Headers    []HeaderExpectation `json:"headers"`
}

type ReplayDiff struct {
	Match        bool `json:"match"`
	StatusMatch  bool `json:"statusMatch"`
	BodyMatch    bool `json:"bodyMatch"`
	HeadersMatch bool `json:"headersMatch"`
}

type ReplayDelivery struct {
	Attempts  int    `json:"attempts"`
	Certain   bool   `json:"certain"`
	ErrorCode string `json:"errorCode,omitempty"`
}

type ReplayResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	Kind          string           `json:"kind"`
	AdapterID     string           `json:"adapterId"`
	Request       FileBinding      `json:"request"`
	Inventory     FileBinding      `json:"inventory"`
	Target        ReplayTarget     `json:"target"`
	Operation     ReplayOperation  `json:"operation"`
	Status        string           `json:"status"`
	Delivery      ReplayDelivery   `json:"delivery"`
	Actual        *ActualResponse  `json:"actual,omitempty"`
	Diff          *ReplayDiff      `json:"diff,omitempty"`
	Limits        ReplayLimits     `json:"limits"`
	Boundaries    ReplayBoundaries `json:"boundaries"`
}

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func BindFile(filePath string, data []byte, maxBytes int) (FileBinding, error) {
	binding := FileBinding{Path: filePath, SHA256: SHA256(data), Bytes: int64(len(data))}
	if err := validateFileBinding(binding, maxBytes, "file"); err != nil {
		return FileBinding{}, err
	}
	return binding, nil
}

func ReplayRequestPath(requestSHA256 string) (string, error) {
	requestSHA256 = strings.TrimSpace(requestSHA256)
	if !sha256Pattern.MatchString(requestSHA256) {
		return "", fmt.Errorf("bounded replay request sha256 is invalid")
	}
	return path.Join(ReplayRequestRoot, requestSHA256+".json"), nil
}

func BindReplayRequest(data []byte) (FileBinding, error) {
	requestSHA256 := SHA256(data)
	requestPath, err := ReplayRequestPath(requestSHA256)
	if err != nil {
		return FileBinding{}, err
	}
	binding, err := BindFile(requestPath, data, MaxReplayRequestBytes)
	if err != nil {
		return FileBinding{}, err
	}
	return binding, nil
}

func ValidateReplayRequestBinding(binding FileBinding) error {
	if err := validateFileBinding(binding, MaxReplayRequestBytes, "bounded replay request"); err != nil {
		return err
	}
	expectedPath, err := ReplayRequestPath(binding.SHA256)
	if err != nil {
		return err
	}
	if binding.Path != expectedPath {
		return fmt.Errorf("bounded replay request path must be content-addressed as %s", expectedPath)
	}
	return nil
}

func CanonicalInventoryBytes(inventory Inventory) ([]byte, error) {
	if err := ValidateInventory(inventory); err != nil {
		return nil, err
	}
	return canonicalBounded(inventory, MaxInventoryBytes, "OpenAPI inventory")
}

func DecodeInventory(data []byte) (Inventory, error) {
	var inventory Inventory
	if err := decodeCanonical(data, MaxInventoryBytes, &inventory, "OpenAPI inventory"); err != nil {
		return Inventory{}, err
	}
	if err := ValidateInventory(inventory); err != nil {
		return Inventory{}, err
	}
	canonical, err := CanonicalInventoryBytes(inventory)
	if err != nil {
		return Inventory{}, err
	}
	if !bytes.Equal(data, canonical) {
		return Inventory{}, fmt.Errorf("OpenAPI inventory is not canonical JSON")
	}
	return inventory, nil
}

func CanonicalReplayRequestBytes(request ReplayRequest) ([]byte, error) {
	if err := ValidateReplayRequest(request); err != nil {
		return nil, err
	}
	return canonicalBounded(request, MaxReplayRequestBytes, "bounded replay request")
}

func DecodeReplayRequest(data []byte) (ReplayRequest, error) {
	var request ReplayRequest
	if err := decodeCanonical(data, MaxReplayRequestBytes, &request, "bounded replay request"); err != nil {
		return ReplayRequest{}, err
	}
	if err := ValidateReplayRequest(request); err != nil {
		return ReplayRequest{}, err
	}
	canonical, err := CanonicalReplayRequestBytes(request)
	if err != nil {
		return ReplayRequest{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ReplayRequest{}, fmt.Errorf("bounded replay request is not canonical JSON")
	}
	return request, nil
}

func CanonicalReplayResultBytes(result ReplayResult) ([]byte, error) {
	if err := ValidateReplayResult(result); err != nil {
		return nil, err
	}
	return canonicalBounded(result, MaxReplayResultBytes, "bounded replay result")
}

func DecodeReplayResult(data []byte) (ReplayResult, error) {
	var result ReplayResult
	if err := decodeCanonical(data, MaxReplayResultBytes, &result, "bounded replay result"); err != nil {
		return ReplayResult{}, err
	}
	if err := ValidateReplayResult(result); err != nil {
		return ReplayResult{}, err
	}
	canonical, err := CanonicalReplayResultBytes(result)
	if err != nil {
		return ReplayResult{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ReplayResult{}, fmt.Errorf("bounded replay result is not canonical JSON")
	}
	return result, nil
}

func ValidateInventory(inventory Inventory) error {
	if inventory.SchemaVersion != SchemaVersion || inventory.Kind != InventoryKind || inventory.AdapterID != InventoryAdapterID {
		return fmt.Errorf("OpenAPI inventory identity is invalid")
	}
	if !strings.HasPrefix(inventory.OpenAPIVersion, "3.0.") && !strings.HasPrefix(inventory.OpenAPIVersion, "3.1.") {
		return fmt.Errorf("OpenAPI inventory requires an OpenAPI 3.x version")
	}
	if err := boundedText(inventory.OpenAPIVersion, "OpenAPI version"); err != nil {
		return err
	}
	if err := validateFileBinding(inventory.Source, MaxOpenAPIBytes, "OpenAPI source"); err != nil {
		return err
	}
	if inventory.Servers == nil || len(inventory.Servers) > MaxServers {
		return fmt.Errorf("OpenAPI inventory servers are missing or exceed %d", MaxServers)
	}
	if inventory.AuthSchemes == nil || len(inventory.AuthSchemes) > MaxAuthSchemes {
		return fmt.Errorf("OpenAPI inventory auth schemes are missing or exceed %d", MaxAuthSchemes)
	}
	if len(inventory.Endpoints) == 0 || len(inventory.Endpoints) > MaxEndpoints {
		return fmt.Errorf("OpenAPI inventory endpoints are missing or exceed %d", MaxEndpoints)
	}
	if inventory.Warnings == nil || len(inventory.Warnings) > 64 {
		return fmt.Errorf("OpenAPI inventory warnings are missing or exceed 64")
	}
	for index, server := range inventory.Servers {
		if err := validateServer(server); err != nil {
			return fmt.Errorf("OpenAPI server %d: %w", index, err)
		}
		if index > 0 && serverKey(inventory.Servers[index-1]) >= serverKey(server) {
			return fmt.Errorf("OpenAPI inventory servers are not strictly sorted and unique")
		}
	}
	authNames := map[string]AuthScheme{}
	for index, scheme := range inventory.AuthSchemes {
		if err := validateAuthScheme(scheme); err != nil {
			return fmt.Errorf("OpenAPI auth scheme %d: %w", index, err)
		}
		if index > 0 && inventory.AuthSchemes[index-1].Name >= scheme.Name {
			return fmt.Errorf("OpenAPI auth schemes are not strictly sorted and unique")
		}
		authNames[scheme.Name] = scheme
	}
	for index, endpoint := range inventory.Endpoints {
		if err := validateEndpoint(endpoint, authNames); err != nil {
			return fmt.Errorf("OpenAPI endpoint %d: %w", index, err)
		}
		if index > 0 && endpointSortKey(inventory.Endpoints[index-1]) >= endpointSortKey(endpoint) {
			return fmt.Errorf("OpenAPI endpoints are not strictly sorted and unique")
		}
	}
	for index, warning := range inventory.Warnings {
		if err := boundedText(warning, "OpenAPI inventory warning"); err != nil {
			return err
		}
		if index > 0 && inventory.Warnings[index-1] >= warning {
			return fmt.Errorf("OpenAPI inventory warnings are not strictly sorted and unique")
		}
	}
	boundary := inventory.Boundaries
	if !boundary.ReadOnlyInput || !boundary.NoNetwork || !boundary.NoSecretsPersisted || !boundary.NoCatalogEntryExec || !boundary.NoAuthorityConfirmed {
		return fmt.Errorf("OpenAPI inventory safety boundaries must all be true")
	}
	return nil
}

func ValidateReplayRequest(request ReplayRequest) error {
	if request.SchemaVersion != SchemaVersion || request.Kind != ReplayRequestKind || request.AdapterID != ReplayAdapterID {
		return fmt.Errorf("bounded replay request identity is invalid")
	}
	if err := validateFileBinding(request.Inventory, MaxInventoryBytes, "OpenAPI inventory"); err != nil {
		return err
	}
	if err := validateReplayTarget(request.Target); err != nil {
		return err
	}
	if err := validateReplayOperation(request.Operation); err != nil {
		return err
	}
	if _, err := replayURL(request.Target, request.Operation); err != nil {
		return err
	}
	if request.Auth != nil {
		if err := boundedText(request.Auth.Scheme, "replay auth scheme"); err != nil {
			return err
		}
		if !authRefPattern.MatchString(request.Auth.AuthRef) {
			return fmt.Errorf("replay authRef must be a STEAMAI_AUTH_ environment reference")
		}
	}
	if err := validateExpectedResponse(request.Expected); err != nil {
		return err
	}
	if err := validateReplayLimits(request.Limits); err != nil {
		return err
	}
	if err := validateReplayBoundaries(request.Boundaries); err != nil {
		return err
	}
	return nil
}

func ValidateReplayResult(result ReplayResult) error {
	if result.SchemaVersion != SchemaVersion || result.Kind != ReplayResultKind || result.AdapterID != ReplayAdapterID {
		return fmt.Errorf("bounded replay result identity is invalid")
	}
	if err := ValidateReplayRequestBinding(result.Request); err != nil {
		return err
	}
	if err := validateFileBinding(result.Inventory, MaxInventoryBytes, "OpenAPI inventory"); err != nil {
		return err
	}
	if err := validateReplayTarget(result.Target); err != nil {
		return err
	}
	if err := validateReplayOperation(result.Operation); err != nil {
		return err
	}
	if err := validateReplayLimits(result.Limits); err != nil {
		return err
	}
	if err := validateReplayBoundaries(result.Boundaries); err != nil {
		return err
	}
	if result.Delivery.Attempts < 0 || result.Delivery.Attempts > 1 {
		return fmt.Errorf("bounded replay result attempts must be 0 or 1")
	}
	switch result.Status {
	case "matched", "different":
		if !result.Delivery.Certain || result.Delivery.Attempts != 1 || result.Delivery.ErrorCode != "" || result.Actual == nil || result.Diff == nil {
			return fmt.Errorf("completed bounded replay result is incomplete")
		}
		if err := validateActualResponse(*result.Actual); err != nil {
			return err
		}
		match := result.Diff.StatusMatch && result.Diff.BodyMatch && result.Diff.HeadersMatch
		if result.Diff.Match != match || (result.Status == "matched") != match {
			return fmt.Errorf("bounded replay result diff status is inconsistent")
		}
	case "delivery-uncertain":
		if result.Delivery.Certain || result.Delivery.Attempts != 1 || result.Delivery.ErrorCode != "delivery-uncertain" || result.Actual != nil || result.Diff != nil {
			return fmt.Errorf("uncertain bounded replay result is inconsistent")
		}
	case "aborted-after-delivery":
		if !result.Delivery.Certain || result.Delivery.Attempts != 1 || (result.Delivery.ErrorCode != "response-body-limit" && result.Delivery.ErrorCode != "response-read") || result.Actual != nil || result.Diff != nil {
			return fmt.Errorf("post-delivery bounded replay abort is inconsistent")
		}
	case "failed-before-delivery":
		if !result.Delivery.Certain || result.Delivery.Attempts != 0 || result.Delivery.ErrorCode == "" || result.Actual != nil || result.Diff != nil {
			return fmt.Errorf("pre-delivery bounded replay failure is inconsistent")
		}
	default:
		return fmt.Errorf("bounded replay result status is invalid")
	}
	return nil
}

func FindEndpoint(inventory Inventory, key string) (Endpoint, bool) {
	for _, endpoint := range inventory.Endpoints {
		if endpoint.Key == key {
			return endpoint, true
		}
	}
	return Endpoint{}, false
}

func FindAuthScheme(inventory Inventory, name string) (AuthScheme, bool) {
	for _, scheme := range inventory.AuthSchemes {
		if scheme.Name == name {
			return scheme, true
		}
	}
	return AuthScheme{}, false
}

func NewReplayRequest(
	inventory Inventory,
	inventoryBinding FileBinding,
	endpointKey string,
	target ReplayTarget,
	concretePath string,
	auth *ReplayAuth,
	expected ExpectedResponse,
	limits ReplayLimits,
) (ReplayRequest, error) {
	if err := ValidateInventory(inventory); err != nil {
		return ReplayRequest{}, err
	}
	if err := validateFileBinding(inventoryBinding, MaxInventoryBytes, "OpenAPI inventory"); err != nil {
		return ReplayRequest{}, err
	}
	inventoryBytes, err := CanonicalInventoryBytes(inventory)
	if err != nil {
		return ReplayRequest{}, err
	}
	if inventoryBinding.Bytes != int64(len(inventoryBytes)) || inventoryBinding.SHA256 != SHA256(inventoryBytes) {
		return ReplayRequest{}, fmt.Errorf("bounded replay inventory binding does not match the canonical inventory")
	}
	endpoint, ok := FindEndpoint(inventory, endpointKey)
	if !ok {
		return ReplayRequest{}, fmt.Errorf("bounded replay endpoint is not present in the inventory")
	}
	if endpoint.Method != "get" && endpoint.Method != "head" && endpoint.Method != "options" {
		return ReplayRequest{}, fmt.Errorf("bounded replay v1 supports only GET, HEAD, or OPTIONS")
	}
	if !concretePathMatchesTemplate(concretePath, endpoint.PathTemplate) {
		return ReplayRequest{}, fmt.Errorf("bounded replay concrete path does not match the inventoried endpoint template")
	}
	if err := validateEndpointAuth(inventory, endpoint, auth); err != nil {
		return ReplayRequest{}, err
	}
	request := ReplayRequest{
		SchemaVersion: SchemaVersion,
		Kind:          ReplayRequestKind,
		AdapterID:     ReplayAdapterID,
		Inventory:     inventoryBinding,
		Target:        target,
		Operation: ReplayOperation{
			Key: endpoint.Key, Method: endpoint.Method, PathTemplate: endpoint.PathTemplate, Path: concretePath,
		},
		Auth:     auth,
		Expected: expected,
		Limits:   limits,
		Boundaries: ReplayBoundaries{
			LoopbackOnly: true, NoAmbientProxy: true, NoRedirects: true, NoRetries: true,
			NoRequestBody: true, NoSecretsPersisted: true, NoCatalogEntryExec: true, NoAuthorityConfirmed: true,
		},
	}
	if err := ValidateReplayRequest(request); err != nil {
		return ReplayRequest{}, err
	}
	return request, nil
}

func DefaultReplayLimits() ReplayLimits {
	return ReplayLimits{RuntimeMillis: 5_000, ResponseBodyBytes: 1 << 20, Requests: 1, Redirects: 0}
}

func validateEndpointAuth(inventory Inventory, endpoint Endpoint, auth *ReplayAuth) error {
	allowsAnonymous := len(endpoint.Security) == 0
	for _, alternative := range endpoint.Security {
		if len(alternative) == 0 {
			allowsAnonymous = true
		}
	}
	if auth == nil {
		if allowsAnonymous {
			return nil
		}
		return fmt.Errorf("inventoried endpoint requires an authRef")
	}
	if strings.TrimSpace(auth.Scheme) == "" || !authRefPattern.MatchString(auth.AuthRef) {
		return fmt.Errorf("bounded replay auth binding is invalid")
	}
	scheme, ok := FindAuthScheme(inventory, auth.Scheme)
	if !ok || !scheme.ReplaySupported {
		return fmt.Errorf("bounded replay auth scheme is absent or unsupported")
	}
	for _, alternative := range endpoint.Security {
		if len(alternative) == 1 && alternative[0] == auth.Scheme {
			return nil
		}
	}
	return fmt.Errorf("bounded replay auth scheme does not satisfy an inventoried single-scheme alternative")
}

func validateFileBinding(binding FileBinding, maxBytes int, label string) error {
	if !validRelativePath(binding.Path) {
		return fmt.Errorf("%s path must be canonical and case-relative", label)
	}
	if !sha256Pattern.MatchString(binding.SHA256) {
		return fmt.Errorf("%s sha256 is invalid", label)
	}
	if binding.Bytes < 1 || binding.Bytes > int64(maxBytes) {
		return fmt.Errorf("%s bytes must be within 1..%d", label, maxBytes)
	}
	return nil
}

func validateServer(server Server) error {
	if server.Scheme != "http" && server.Scheme != "https" {
		return fmt.Errorf("server scheme must be http or https")
	}
	if err := validateHost(server.Host); err != nil {
		return err
	}
	if server.Port < 1 || server.Port > 65535 {
		return fmt.Errorf("server port is invalid")
	}
	if !validURLPath(server.BasePath, true) {
		return fmt.Errorf("server basePath is invalid")
	}
	return nil
}

func validateAuthScheme(scheme AuthScheme) error {
	if err := boundedText(scheme.Name, "auth scheme name"); err != nil {
		return err
	}
	switch scheme.Type {
	case "apiKey":
		if scheme.In != "header" && scheme.In != "query" && scheme.In != "cookie" {
			return fmt.Errorf("apiKey location is invalid")
		}
		if err := boundedText(scheme.Parameter, "apiKey parameter"); err != nil {
			return err
		}
		if scheme.Scheme != "" || scheme.ReplaySupported != (scheme.In == "header") {
			return fmt.Errorf("apiKey replay support is inconsistent")
		}
	case "http":
		if scheme.In != "" || scheme.Parameter != "" || (scheme.Scheme != "bearer" && scheme.Scheme != "basic") || scheme.ReplaySupported != (scheme.Scheme == "bearer") {
			return fmt.Errorf("HTTP auth scheme is invalid or replay support is inconsistent")
		}
	case "oauth2", "openIdConnect", "mutualTLS":
		if scheme.In != "" || scheme.Parameter != "" || scheme.Scheme != "" || scheme.ReplaySupported {
			return fmt.Errorf("non-header auth scheme must not claim replay support")
		}
	default:
		return fmt.Errorf("auth scheme type is invalid")
	}
	return nil
}

func validateEndpoint(endpoint Endpoint, authNames map[string]AuthScheme) error {
	if err := boundedText(endpoint.Key, "endpoint key"); err != nil {
		return err
	}
	if endpoint.OperationID != "" {
		if err := boundedText(endpoint.OperationID, "operationId"); err != nil {
			return err
		}
	}
	if _, ok := methodOrder[endpoint.Method]; !ok {
		return fmt.Errorf("endpoint method is invalid")
	}
	if !validPathTemplate(endpoint.PathTemplate) || endpoint.Key != endpoint.Method+" "+endpoint.PathTemplate {
		return fmt.Errorf("endpoint key or path template is invalid")
	}
	if endpoint.Parameters == nil || len(endpoint.Parameters) > MaxParameters {
		return fmt.Errorf("endpoint parameters are missing or exceed %d", MaxParameters)
	}
	for index, parameter := range endpoint.Parameters {
		if err := boundedText(parameter.Name, "parameter name"); err != nil {
			return err
		}
		if parameter.In != "path" && parameter.In != "query" && parameter.In != "header" && parameter.In != "cookie" {
			return fmt.Errorf("parameter location is invalid")
		}
		if parameter.In == "path" && !parameter.Required {
			return fmt.Errorf("path parameters must be required")
		}
		if index > 0 && parameterKey(endpoint.Parameters[index-1]) >= parameterKey(parameter) {
			return fmt.Errorf("endpoint parameters are not strictly sorted and unique")
		}
	}
	if endpoint.RequestMediaTypes == nil || len(endpoint.RequestMediaTypes) > MaxMediaTypes {
		return fmt.Errorf("request media types are missing or exceed %d", MaxMediaTypes)
	}
	for index, mediaType := range endpoint.RequestMediaTypes {
		if err := boundedText(mediaType, "request media type"); err != nil {
			return err
		}
		if index > 0 && endpoint.RequestMediaTypes[index-1] >= mediaType {
			return fmt.Errorf("request media types are not strictly sorted and unique")
		}
	}
	if endpoint.Security == nil || len(endpoint.Security) > 32 {
		return fmt.Errorf("endpoint security alternatives are missing or exceed 32")
	}
	previous := ""
	for index, alternative := range endpoint.Security {
		if alternative == nil || len(alternative) > 16 || !slices.IsSorted(alternative) {
			return fmt.Errorf("endpoint security alternative is invalid")
		}
		for itemIndex, name := range alternative {
			if _, ok := authNames[name]; !ok {
				return fmt.Errorf("endpoint references unknown auth scheme %q", name)
			}
			if itemIndex > 0 && alternative[itemIndex-1] == name {
				return fmt.Errorf("endpoint security alternative contains duplicates")
			}
		}
		key := strings.Join(alternative, "\x00")
		if index > 0 && previous >= key {
			return fmt.Errorf("endpoint security alternatives are not strictly sorted and unique")
		}
		previous = key
	}
	return nil
}

func validateReplayTarget(target ReplayTarget) error {
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("bounded replay target scheme must be http or https")
	}
	ip := net.ParseIP(target.Host)
	if ip == nil || !ip.IsLoopback() || target.Host != ip.String() {
		return fmt.Errorf("bounded replay target host must be a canonical loopback IP literal")
	}
	if target.Port < 1 || target.Port > 65535 {
		return fmt.Errorf("bounded replay target port is invalid")
	}
	if !validURLPath(target.BasePath, true) {
		return fmt.Errorf("bounded replay target basePath is invalid")
	}
	return nil
}

func validateReplayOperation(operation ReplayOperation) error {
	if operation.Method != "get" && operation.Method != "head" && operation.Method != "options" {
		return fmt.Errorf("bounded replay operation method is unsupported")
	}
	if !validPathTemplate(operation.PathTemplate) || operation.Key != operation.Method+" "+operation.PathTemplate {
		return fmt.Errorf("bounded replay operation binding is invalid")
	}
	if !concretePathMatchesTemplate(operation.Path, operation.PathTemplate) {
		return fmt.Errorf("bounded replay concrete path does not match its template")
	}
	return nil
}

func validateExpectedResponse(expected ExpectedResponse) error {
	if expected.StatusCode < 100 || expected.StatusCode > 599 {
		return fmt.Errorf("expected response status code is invalid")
	}
	if err := validateDigestExpectation(expected.Body, MaxResponseBodyBytes, "expected response body"); err != nil {
		return err
	}
	if expected.Headers == nil || len(expected.Headers) > 8 {
		return fmt.Errorf("expected response headers are missing or exceed 8")
	}
	for index, header := range expected.Headers {
		if header.Name != "content-type" {
			return fmt.Errorf("bounded replay v1 permits only content-type response header expectations")
		}
		if err := validateDigestExpectation(DigestExpectation{SHA256: header.SHA256, Bytes: header.Bytes}, MaxStringBytes, "expected response header"); err != nil {
			return err
		}
		if index > 0 && expected.Headers[index-1].Name >= header.Name {
			return fmt.Errorf("expected response headers are not strictly sorted and unique")
		}
	}
	return nil
}

func validateActualResponse(actual ActualResponse) error {
	if actual.StatusCode < 100 || actual.StatusCode > 599 {
		return fmt.Errorf("actual response status code is invalid")
	}
	if err := validateDigestExpectation(actual.Body, MaxResponseBodyBytes, "actual response body"); err != nil {
		return err
	}
	if actual.Headers == nil || len(actual.Headers) > 8 {
		return fmt.Errorf("actual response headers are missing or exceed 8")
	}
	for index, header := range actual.Headers {
		if header.Name != "content-type" {
			return fmt.Errorf("bounded replay result contains an unsupported header digest")
		}
		if err := validateDigestExpectation(DigestExpectation{SHA256: header.SHA256, Bytes: header.Bytes}, MaxStringBytes, "actual response header"); err != nil {
			return err
		}
		if index > 0 && actual.Headers[index-1].Name >= header.Name {
			return fmt.Errorf("actual response headers are not strictly sorted and unique")
		}
	}
	return nil
}

func validateDigestExpectation(expectation DigestExpectation, maxBytes int, label string) error {
	if !sha256Pattern.MatchString(expectation.SHA256) || expectation.Bytes < 0 || expectation.Bytes > int64(maxBytes) {
		return fmt.Errorf("%s digest or byte count is invalid", label)
	}
	return nil
}

func validateReplayLimits(limits ReplayLimits) error {
	if limits.RuntimeMillis < 1 || limits.RuntimeMillis > MaxRuntimeMillis || limits.ResponseBodyBytes < 1 || limits.ResponseBodyBytes > MaxResponseBodyBytes || limits.Requests != 1 || limits.Redirects != 0 {
		return fmt.Errorf("bounded replay limits require one request, zero redirects, and bounded runtime/body bytes")
	}
	return nil
}

func validateReplayBoundaries(boundaries ReplayBoundaries) error {
	if !boundaries.LoopbackOnly || !boundaries.NoAmbientProxy || !boundaries.NoRedirects || !boundaries.NoRetries || !boundaries.NoRequestBody || !boundaries.NoSecretsPersisted || !boundaries.NoCatalogEntryExec || !boundaries.NoAuthorityConfirmed {
		return fmt.Errorf("bounded replay safety boundaries must all be true")
	}
	return nil
}

func validateHost(host string) error {
	if host == "" || host != strings.ToLower(host) || strings.TrimSpace(host) != host || len(host) > 253 {
		return fmt.Errorf("server host is invalid")
	}
	if ip := net.ParseIP(host); ip != nil {
		if host != ip.String() {
			return fmt.Errorf("server IP host is not canonical")
		}
		return nil
	}
	if !hostPattern.MatchString(host) || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return fmt.Errorf("server DNS host is invalid")
	}
	return nil
}

func validRelativePath(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) && len(value) <= 1024 && !strings.ContainsAny(value, `:\\`) && !strings.ContainsRune(value, 0) && !strings.HasPrefix(value, "/") && path.Clean(value) == value && value != "." && !strings.HasPrefix(value, "../")
}

func validURLPath(value string, allowRoot bool) bool {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || len(value) > 2048 || !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#\\") || strings.ContainsRune(value, 0) {
		return false
	}
	if value == "/" {
		return allowRoot
	}
	return !strings.Contains(value, "//") && path.Clean(value) == value
}

func validPathTemplate(value string) bool {
	if !validURLPath(value, true) {
		return false
	}
	for segment := range strings.SplitSeq(strings.TrimPrefix(value, "/"), "/") {
		if strings.ContainsAny(segment, "{}") && !(strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") && strings.Count(segment, "{") == 1 && strings.Count(segment, "}") == 1 && len(segment) > 2) {
			return false
		}
	}
	return true
}

func concretePathMatchesTemplate(concrete, template string) bool {
	if !validURLPath(concrete, true) || !validPathTemplate(template) {
		return false
	}
	concreteParts := strings.Split(strings.TrimPrefix(concrete, "/"), "/")
	templateParts := strings.Split(strings.TrimPrefix(template, "/"), "/")
	if len(concreteParts) != len(templateParts) {
		return false
	}
	for index := range concreteParts {
		if strings.HasPrefix(templateParts[index], "{") && strings.HasSuffix(templateParts[index], "}") {
			if concreteParts[index] == "" {
				return false
			}
			continue
		}
		if concreteParts[index] != templateParts[index] {
			return false
		}
	}
	return true
}

func boundedText(value, label string) error {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) || len(value) > MaxStringBytes || strings.ContainsRune(value, 0) {
		return fmt.Errorf("%s is empty, non-canonical, or exceeds %d bytes", label, MaxStringBytes)
	}
	return nil
}

func canonicalBounded(value any, maxBytes int, label string) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if len(data) > maxBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", label, maxBytes)
	}
	return data, nil
}

func decodeCanonical(data []byte, maxBytes int, target any, label string) error {
	if len(data) == 0 || len(data) > maxBytes {
		return fmt.Errorf("%s size is invalid", label)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%s must contain exactly one JSON object", label)
	}
	return nil
}

func serverKey(server Server) string {
	return server.Scheme + "\x00" + server.Host + "\x00" + strconv.Itoa(server.Port) + "\x00" + server.BasePath
}

func endpointSortKey(endpoint Endpoint) string {
	order := methodOrder[endpoint.Method]
	return endpoint.PathTemplate + "\x00" + fmt.Sprintf("%02d", order) + "\x00" + endpoint.Method
}

func parameterKey(parameter Parameter) string {
	return parameter.In + "\x00" + strings.ToLower(parameter.Name) + "\x00" + parameter.Name
}
