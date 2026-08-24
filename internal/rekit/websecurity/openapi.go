package websecurity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const maxJSONDepth = 128

func ImportOpenAPI(source FileBinding, data []byte) (Inventory, error) {
	if err := validateFileBinding(source, MaxOpenAPIBytes, "OpenAPI source"); err != nil {
		return Inventory{}, err
	}
	if int64(len(data)) != source.Bytes || SHA256(data) != source.SHA256 {
		return Inventory{}, fmt.Errorf("OpenAPI source binding does not match input bytes")
	}
	rootValue, err := decodeUniqueJSON(data)
	if err != nil {
		return Inventory{}, fmt.Errorf("decode OpenAPI JSON: %w", err)
	}
	root, ok := rootValue.(map[string]any)
	if !ok {
		return Inventory{}, fmt.Errorf("OpenAPI source must contain one JSON object")
	}
	version, err := requiredString(root, "openapi", "OpenAPI version")
	if err != nil {
		return Inventory{}, err
	}
	if !strings.HasPrefix(version, "3.0.") && !strings.HasPrefix(version, "3.1.") {
		return Inventory{}, fmt.Errorf("OpenAPI importer supports only OpenAPI 3.0.x or 3.1.x JSON")
	}

	inventory := Inventory{
		SchemaVersion:  SchemaVersion,
		Kind:           InventoryKind,
		AdapterID:      InventoryAdapterID,
		OpenAPIVersion: version,
		Source:         source,
		Servers:        []Server{},
		AuthSchemes:    []AuthScheme{},
		Endpoints:      []Endpoint{},
		Warnings:       []string{},
		Boundaries: InventoryBoundaries{
			ReadOnlyInput: true, NoNetwork: true, NoSecretsPersisted: true,
			NoCatalogEntryExec: true, NoAuthorityConfirmed: true,
		},
	}
	warnings := map[string]bool{}
	if err := importServers(root, &inventory, warnings); err != nil {
		return Inventory{}, err
	}
	if err := importAuthSchemes(root, &inventory); err != nil {
		return Inventory{}, err
	}
	globalSecurity, present, err := importSecurity(root, "OpenAPI root")
	if err != nil {
		return Inventory{}, err
	}
	if !present {
		globalSecurity = [][]string{}
	}
	if err := importEndpoints(root, globalSecurity, &inventory, warnings); err != nil {
		return Inventory{}, err
	}
	for warning := range warnings {
		inventory.Warnings = append(inventory.Warnings, warning)
	}
	sort.Slice(inventory.Servers, func(i, j int) bool { return serverKey(inventory.Servers[i]) < serverKey(inventory.Servers[j]) })
	inventory.Servers = uniqueServers(inventory.Servers)
	sort.Slice(inventory.AuthSchemes, func(i, j int) bool { return inventory.AuthSchemes[i].Name < inventory.AuthSchemes[j].Name })
	sort.Slice(inventory.Endpoints, func(i, j int) bool {
		return endpointSortKey(inventory.Endpoints[i]) < endpointSortKey(inventory.Endpoints[j])
	})
	sort.Strings(inventory.Warnings)
	inventory.Warnings = uniqueStrings(inventory.Warnings)
	if err := ValidateInventory(inventory); err != nil {
		return Inventory{}, err
	}
	return inventory, nil
}

func importServers(root map[string]any, inventory *Inventory, warnings map[string]bool) error {
	value, present := root["servers"]
	if !present {
		return nil
	}
	servers, ok := value.([]any)
	if !ok {
		return fmt.Errorf("OpenAPI servers must be an array")
	}
	if len(servers) > MaxServers {
		return fmt.Errorf("OpenAPI servers exceed %d", MaxServers)
	}
	for index, item := range servers {
		object, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("OpenAPI server %d must be an object", index)
		}
		rawURL, err := requiredString(object, "url", fmt.Sprintf("OpenAPI server %d url", index))
		if err != nil {
			return err
		}
		if strings.ContainsAny(rawURL, "{}") {
			warnings[fmt.Sprintf("server %d uses variables and was omitted from exact replay inventory", index)] = true
			continue
		}
		parsed, err := url.Parse(rawURL)
		if err != nil || !parsed.IsAbs() || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
			warnings[fmt.Sprintf("server %d is not an exact absolute HTTP(S) origin and was omitted", index)] = true
			continue
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" {
			warnings[fmt.Sprintf("server %d is not HTTP(S) and was omitted", index)] = true
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		port := 80
		if scheme == "https" {
			port = 443
		}
		if parsed.Port() != "" {
			parsedPort, err := strconv.Atoi(parsed.Port())
			if err != nil || parsedPort < 1 || parsedPort > 65535 {
				return fmt.Errorf("OpenAPI server %d port is invalid", index)
			}
			port = parsedPort
		}
		basePath := parsed.EscapedPath()
		if basePath == "" {
			basePath = "/"
		}
		server := Server{Scheme: scheme, Host: host, Port: port, BasePath: basePath}
		if err := validateServer(server); err != nil {
			return fmt.Errorf("OpenAPI server %d: %w", index, err)
		}
		inventory.Servers = append(inventory.Servers, server)
	}
	return nil
}

func importAuthSchemes(root map[string]any, inventory *Inventory) error {
	componentsValue, present := root["components"]
	if !present {
		return nil
	}
	components, ok := componentsValue.(map[string]any)
	if !ok {
		return fmt.Errorf("OpenAPI components must be an object")
	}
	schemesValue, present := components["securitySchemes"]
	if !present {
		return nil
	}
	schemes, ok := schemesValue.(map[string]any)
	if !ok {
		return fmt.Errorf("OpenAPI components.securitySchemes must be an object")
	}
	if len(schemes) > MaxAuthSchemes {
		return fmt.Errorf("OpenAPI auth schemes exceed %d", MaxAuthSchemes)
	}
	for name, value := range schemes {
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("OpenAPI auth scheme %q must be an object", name)
		}
		if _, ref := object["$ref"]; ref {
			return fmt.Errorf("OpenAPI auth scheme %q uses unsupported $ref", name)
		}
		typeName, err := requiredString(object, "type", "OpenAPI auth scheme type")
		if err != nil {
			return fmt.Errorf("OpenAPI auth scheme %q: %w", name, err)
		}
		scheme := AuthScheme{Name: name, Type: typeName}
		switch typeName {
		case "apiKey":
			scheme.In, err = requiredString(object, "in", "OpenAPI apiKey location")
			if err != nil {
				return fmt.Errorf("OpenAPI auth scheme %q: %w", name, err)
			}
			scheme.Parameter, err = requiredString(object, "name", "OpenAPI apiKey parameter")
			if err != nil {
				return fmt.Errorf("OpenAPI auth scheme %q: %w", name, err)
			}
			scheme.ReplaySupported = scheme.In == "header"
		case "http":
			scheme.Scheme, err = requiredString(object, "scheme", "OpenAPI HTTP auth scheme")
			if err != nil {
				return fmt.Errorf("OpenAPI auth scheme %q: %w", name, err)
			}
			scheme.Scheme = strings.ToLower(scheme.Scheme)
			scheme.ReplaySupported = scheme.Scheme == "bearer"
		case "oauth2", "openIdConnect", "mutualTLS":
		default:
			return fmt.Errorf("OpenAPI auth scheme %q has unsupported type %q", name, typeName)
		}
		if err := validateAuthScheme(scheme); err != nil {
			return fmt.Errorf("OpenAPI auth scheme %q: %w", name, err)
		}
		inventory.AuthSchemes = append(inventory.AuthSchemes, scheme)
	}
	return nil
}

func importEndpoints(root map[string]any, globalSecurity [][]string, inventory *Inventory, warnings map[string]bool) error {
	pathsValue, present := root["paths"]
	if !present {
		return fmt.Errorf("OpenAPI source has no paths")
	}
	paths, ok := pathsValue.(map[string]any)
	if !ok {
		return fmt.Errorf("OpenAPI paths must be an object")
	}
	if len(paths) > MaxEndpoints {
		return fmt.Errorf("OpenAPI paths exceed %d", MaxEndpoints)
	}
	for pathTemplate, pathValue := range paths {
		if !validPathTemplate(pathTemplate) {
			return fmt.Errorf("OpenAPI path %q is not canonical", pathTemplate)
		}
		pathItem, ok := pathValue.(map[string]any)
		if !ok {
			return fmt.Errorf("OpenAPI path item %q must be an object", pathTemplate)
		}
		if _, ref := pathItem["$ref"]; ref {
			return fmt.Errorf("OpenAPI path item %q uses unsupported $ref", pathTemplate)
		}
		pathParameters, err := importParameters(pathItem["parameters"], "path item "+pathTemplate)
		if err != nil {
			return err
		}
		for method := range methodOrder {
			operationValue, present := pathItem[method]
			if !present {
				continue
			}
			operation, ok := operationValue.(map[string]any)
			if !ok {
				return fmt.Errorf("OpenAPI operation %s %s must be an object", method, pathTemplate)
			}
			if _, ref := operation["$ref"]; ref {
				return fmt.Errorf("OpenAPI operation %s %s uses unsupported $ref", method, pathTemplate)
			}
			operationParameters, err := importParameters(operation["parameters"], "operation "+method+" "+pathTemplate)
			if err != nil {
				return err
			}
			parameters := mergeParameters(pathParameters, operationParameters)
			security, present, err := importSecurity(operation, "operation "+method+" "+pathTemplate)
			if err != nil {
				return err
			}
			if !present {
				security = cloneSecurity(globalSecurity)
			}
			requestBodyRequired, mediaTypes, err := importRequestBody(operation, method+" "+pathTemplate)
			if err != nil {
				return err
			}
			operationID := ""
			if value, exists := operation["operationId"]; exists {
				operationID, ok = value.(string)
				if !ok {
					return fmt.Errorf("OpenAPI operationId for %s %s must be a string", method, pathTemplate)
				}
				if err := boundedText(operationID, "operationId"); err != nil {
					return err
				}
			}
			endpoint := Endpoint{
				Key: method + " " + pathTemplate, OperationID: operationID, Method: method,
				PathTemplate: pathTemplate, Parameters: parameters,
				RequestBodyRequired: requestBodyRequired, RequestMediaTypes: mediaTypes, Security: security,
			}
			inventory.Endpoints = append(inventory.Endpoints, endpoint)
			if method != "get" && method != "head" && method != "options" {
				warnings["state-changing operations are inventoried but bounded replay v1 supports only GET, HEAD, and OPTIONS"] = true
			}
			if len(inventory.Endpoints) > MaxEndpoints {
				return fmt.Errorf("OpenAPI operations exceed %d", MaxEndpoints)
			}
		}
	}
	if len(inventory.Endpoints) == 0 {
		return fmt.Errorf("OpenAPI source contains no supported HTTP operations")
	}
	return nil
}

func importParameters(value any, context string) ([]Parameter, error) {
	if value == nil {
		return []Parameter{}, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("OpenAPI parameters for %s must be an array", context)
	}
	if len(items) > MaxParameters {
		return nil, fmt.Errorf("OpenAPI parameters for %s exceed %d", context, MaxParameters)
	}
	parameters := make([]Parameter, 0, len(items))
	for index, value := range items {
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("OpenAPI parameter %d for %s must be an object", index, context)
		}
		if _, ref := object["$ref"]; ref {
			return nil, fmt.Errorf("OpenAPI parameter %d for %s uses unsupported $ref", index, context)
		}
		name, err := requiredString(object, "name", "OpenAPI parameter name")
		if err != nil {
			return nil, err
		}
		location, err := requiredString(object, "in", "OpenAPI parameter location")
		if err != nil {
			return nil, err
		}
		if location == "header" {
			name = strings.ToLower(name)
		}
		required := false
		if raw, present := object["required"]; present {
			required, ok = raw.(bool)
			if !ok {
				return nil, fmt.Errorf("OpenAPI parameter required flag for %s must be boolean", context)
			}
		}
		if location == "path" && !required {
			return nil, fmt.Errorf("OpenAPI path parameter %q must be required", name)
		}
		parameters = append(parameters, Parameter{Name: name, In: location, Required: required})
	}
	sort.Slice(parameters, func(i, j int) bool { return parameterKey(parameters[i]) < parameterKey(parameters[j]) })
	for index := 1; index < len(parameters); index++ {
		if parameterKey(parameters[index-1]) == parameterKey(parameters[index]) {
			return nil, fmt.Errorf("OpenAPI parameters for %s contain duplicate name/location", context)
		}
	}
	return parameters, nil
}

func importSecurity(object map[string]any, context string) ([][]string, bool, error) {
	value, present := object["security"]
	if !present {
		return nil, false, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, true, fmt.Errorf("OpenAPI security for %s must be an array", context)
	}
	if len(items) > 32 {
		return nil, true, fmt.Errorf("OpenAPI security for %s exceeds 32 alternatives", context)
	}
	security := make([][]string, 0, len(items))
	for index, item := range items {
		requirement, ok := item.(map[string]any)
		if !ok {
			return nil, true, fmt.Errorf("OpenAPI security alternative %d for %s must be an object", index, context)
		}
		names := make([]string, 0, len(requirement))
		for name, scopesValue := range requirement {
			scopes, ok := scopesValue.([]any)
			if !ok {
				return nil, true, fmt.Errorf("OpenAPI security scopes for %q must be an array", name)
			}
			for _, scope := range scopes {
				if _, ok := scope.(string); !ok {
					return nil, true, fmt.Errorf("OpenAPI security scopes for %q must contain strings", name)
				}
			}
			names = append(names, name)
		}
		sort.Strings(names)
		security = append(security, names)
	}
	sort.Slice(security, func(i, j int) bool { return strings.Join(security[i], "\x00") < strings.Join(security[j], "\x00") })
	for index := 1; index < len(security); index++ {
		if strings.Join(security[index-1], "\x00") == strings.Join(security[index], "\x00") {
			return nil, true, fmt.Errorf("OpenAPI security for %s contains duplicate alternatives", context)
		}
	}
	return security, true, nil
}

func importRequestBody(operation map[string]any, context string) (bool, []string, error) {
	value, present := operation["requestBody"]
	if !present {
		return false, []string{}, nil
	}
	body, ok := value.(map[string]any)
	if !ok {
		return false, nil, fmt.Errorf("OpenAPI requestBody for %s must be an object", context)
	}
	if _, ref := body["$ref"]; ref {
		return false, nil, fmt.Errorf("OpenAPI requestBody for %s uses unsupported $ref", context)
	}
	required := false
	if value, present := body["required"]; present {
		required, ok = value.(bool)
		if !ok {
			return false, nil, fmt.Errorf("OpenAPI requestBody required flag for %s must be boolean", context)
		}
	}
	mediaTypes := []string{}
	if value, present := body["content"]; present {
		content, ok := value.(map[string]any)
		if !ok {
			return false, nil, fmt.Errorf("OpenAPI requestBody content for %s must be an object", context)
		}
		if len(content) > MaxMediaTypes {
			return false, nil, fmt.Errorf("OpenAPI requestBody media types for %s exceed %d", context, MaxMediaTypes)
		}
		for mediaType := range content {
			mediaType = strings.ToLower(strings.TrimSpace(mediaType))
			if err := boundedText(mediaType, "request media type"); err != nil {
				return false, nil, err
			}
			mediaTypes = append(mediaTypes, mediaType)
		}
	}
	sort.Strings(mediaTypes)
	mediaTypes = uniqueStrings(mediaTypes)
	return required, mediaTypes, nil
}

func requiredString(object map[string]any, key, label string) (string, error) {
	value, present := object[key]
	if !present {
		return "", fmt.Errorf("%s is missing", label)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", label)
	}
	if err := boundedText(text, label); err != nil {
		return "", err
	}
	return text, nil
}

func mergeParameters(pathParameters, operationParameters []Parameter) []Parameter {
	merged := map[string]Parameter{}
	for _, parameter := range pathParameters {
		merged[parameterKey(parameter)] = parameter
	}
	for _, parameter := range operationParameters {
		merged[parameterKey(parameter)] = parameter
	}
	out := make([]Parameter, 0, len(merged))
	for _, parameter := range merged {
		out = append(out, parameter)
	}
	sort.Slice(out, func(i, j int) bool { return parameterKey(out[i]) < parameterKey(out[j]) })
	return out
}

func cloneSecurity(security [][]string) [][]string {
	out := make([][]string, len(security))
	for index := range security {
		out[index] = append([]string{}, security[index]...)
	}
	return out
}

func uniqueServers(values []Server) []Server {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if serverKey(out[len(out)-1]) != serverKey(value) {
			out = append(out, value)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}

func decodeUniqueJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	value, err := decodeUniqueJSONValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("OpenAPI JSON must contain exactly one value")
		}
		return nil, fmt.Errorf("decode trailing OpenAPI JSON: %w", err)
	}
	return value, nil
}

func decodeUniqueJSONValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, fmt.Errorf("OpenAPI JSON exceeds maximum nesting depth")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("OpenAPI JSON object key is not a string")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("OpenAPI JSON contains duplicate key %q", key)
			}
			value, err := decodeUniqueJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, fmt.Errorf("OpenAPI JSON object is not closed")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeUniqueJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, fmt.Errorf("OpenAPI JSON array is not closed")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("OpenAPI JSON contains unexpected delimiter")
	}
}
