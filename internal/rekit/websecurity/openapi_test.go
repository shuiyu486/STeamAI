package websecurity

import (
	"bytes"
	"strings"
	"testing"
)

const syntheticOpenAPI = `{
  "openapi": "3.1.0",
  "info": {
    "title": "Loopback fixture",
    "version": "1.0.0",
    "description": "Authorization: Bearer MUST_NOT_PERSIST"
  },
  "servers": [
    {"url": "http://127.0.0.1:18080/api"},
    {"url": "https://example.invalid/v1"},
    {"url": "https://{tenant}.example.invalid/v1", "variables": {"tenant": {"default": "SECRET_TENANT"}}}
  ],
  "components": {
    "securitySchemes": {
      "apiKey": {"type": "apiKey", "in": "header", "name": "X-API-Key", "description": "SECRET_API_KEY"},
      "bearer": {"type": "http", "scheme": "bearer", "bearerFormat": "JWT_SECRET"}
    }
  },
  "security": [{"bearer": []}],
  "paths": {
    "/items/{id}": {
      "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string", "default": "SECRET_ID"}}],
      "get": {
        "operationId": "getItem",
        "parameters": [{"name": "X-Trace", "in": "header", "required": false}],
        "responses": {"200": {"description": "ok", "content": {"application/json": {"example": {"token": "SECRET_RESPONSE"}}}}}
      },
      "post": {
        "operationId": "updateItem",
        "security": [{"apiKey": []}],
        "requestBody": {"required": true, "content": {"application/json": {"example": {"password": "SECRET_BODY"}}}},
        "responses": {"204": {"description": "done"}}
      }
    },
    "/health": {
      "get": {
        "operationId": "health",
        "security": [],
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`

func TestImportOpenAPIProducesCanonicalSecretFreeInventory(t *testing.T) {
	source, err := BindFile("inputs/openapi.json", []byte(syntheticOpenAPI), MaxOpenAPIBytes)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := ImportOpenAPI(source, []byte(syntheticOpenAPI))
	if err != nil {
		t.Fatal(err)
	}
	if inventory.OpenAPIVersion != "3.1.0" || len(inventory.Servers) != 2 || len(inventory.AuthSchemes) != 2 || len(inventory.Endpoints) != 3 {
		t.Fatalf("unexpected OpenAPI inventory: %+v", inventory)
	}
	if inventory.Servers[0] != (Server{Scheme: "http", Host: "127.0.0.1", Port: 18080, BasePath: "/api"}) {
		t.Fatalf("unexpected first server: %+v", inventory.Servers[0])
	}
	if inventory.Endpoints[0].Key != "get /health" || inventory.Endpoints[1].Key != "get /items/{id}" || inventory.Endpoints[2].Key != "post /items/{id}" {
		t.Fatalf("endpoints are not canonical: %+v", inventory.Endpoints)
	}
	item := inventory.Endpoints[1]
	if item.OperationID != "getItem" || len(item.Parameters) != 2 || item.Parameters[0].In != "header" || item.Parameters[0].Name != "x-trace" || len(item.Security) != 1 || item.Security[0][0] != "bearer" {
		t.Fatalf("GET endpoint inventory drifted: %+v", item)
	}
	post := inventory.Endpoints[2]
	if !post.RequestBodyRequired || len(post.RequestMediaTypes) != 1 || post.RequestMediaTypes[0] != "application/json" || post.Security[0][0] != "apiKey" {
		t.Fatalf("POST endpoint inventory drifted: %+v", post)
	}
	if len(inventory.Warnings) != 2 || !inventory.Boundaries.ReadOnlyInput || !inventory.Boundaries.NoNetwork || !inventory.Boundaries.NoSecretsPersisted {
		t.Fatalf("inventory warnings or boundaries drifted: %+v", inventory)
	}
	canonical, err := CanonicalInventoryBytes(inventory)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"MUST_NOT_PERSIST", "SECRET_TENANT", "SECRET_API_KEY", "JWT_SECRET", "SECRET_ID", "SECRET_RESPONSE", "SECRET_BODY"} {
		if bytes.Contains(canonical, []byte(secret)) {
			t.Fatalf("canonical inventory retained source-only value %q:\n%s", secret, canonical)
		}
	}
	decoded, err := DecodeInventory(canonical)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalInventoryBytes(decoded)
	if err != nil || !bytes.Equal(canonical, second) {
		t.Fatalf("canonical inventory round trip drifted: err=%v", err)
	}
}

func TestImportOpenAPIRejectsAmbiguousOrUnsupportedInputs(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
		want string
	}{
		{name: "duplicate-key", data: `{"openapi":"3.1.0","openapi":"3.0.3","paths":{"/health":{"get":{}}}}`, want: "duplicate key"},
		{name: "swagger-two", data: `{"swagger":"2.0","paths":{"/health":{"get":{}}}}`, want: "version is missing"},
		{name: "external-ref", data: `{"openapi":"3.1.0","paths":{"/health":{"$ref":"other.json#/x"}}}`, want: "unsupported $ref"},
		{name: "path-param-not-required", data: `{"openapi":"3.1.0","paths":{"/items/{id}":{"get":{"parameters":[{"name":"id","in":"path"}]}}}}`, want: "must be required"},
		{name: "trailing", data: `{"openapi":"3.1.0","paths":{"/health":{"get":{}}}} {}`, want: "exactly one"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, err := BindFile("inputs/openapi.json", []byte(test.data), MaxOpenAPIBytes)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ImportOpenAPI(source, []byte(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want %q", err, test.want)
			}
		})
	}
}

func TestNewReplayRequestBindsInventoriedLoopbackOperationWithoutSecrets(t *testing.T) {
	source, err := BindFile("inputs/openapi.json", []byte(syntheticOpenAPI), MaxOpenAPIBytes)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := ImportOpenAPI(source, []byte(syntheticOpenAPI))
	if err != nil {
		t.Fatal(err)
	}
	inventoryBytes, err := CanonicalInventoryBytes(inventory)
	if err != nil {
		t.Fatal(err)
	}
	inventoryBinding, err := BindFile("workspace/main/openapi-inventory.json", inventoryBytes, MaxInventoryBytes)
	if err != nil {
		t.Fatal(err)
	}
	expectedBody := []byte(`{"id":"42","ok":true}` + "\n")
	contentType := []byte("application/json")
	request, err := NewReplayRequest(
		inventory,
		inventoryBinding,
		"get /items/{id}",
		ReplayTarget{Scheme: "http", Host: "127.0.0.1", Port: 18080, BasePath: "/api"},
		"/items/42",
		&ReplayAuth{Scheme: "bearer", AuthRef: "STEAMAI_AUTH_LOOPBACK_BEARER"},
		ExpectedResponse{
			StatusCode: 200,
			Body:       DigestExpectation{SHA256: SHA256(expectedBody), Bytes: int64(len(expectedBody))},
			Headers:    []HeaderExpectation{{Name: "content-type", SHA256: SHA256(contentType), Bytes: int64(len(contentType))}},
		},
		DefaultReplayLimits(),
	)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalReplayRequestBytes(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(canonical, []byte(`"authRef": "STEAMAI_AUTH_LOOPBACK_BEARER"`)) || bytes.Contains(canonical, []byte("Bearer fixture-secret")) || !request.Boundaries.NoAmbientProxy || !request.Boundaries.NoRetries || !request.Boundaries.NoRequestBody {
		t.Fatalf("bounded replay request is not secret-free or bounded:\n%s", canonical)
	}
	if _, err := DecodeReplayRequest(canonical); err != nil {
		t.Fatal(err)
	}
}

func TestNewReplayRequestRejectsNonLoopbackWritesAndUnsafeAuth(t *testing.T) {
	source, err := BindFile("inputs/openapi.json", []byte(syntheticOpenAPI), MaxOpenAPIBytes)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := ImportOpenAPI(source, []byte(syntheticOpenAPI))
	if err != nil {
		t.Fatal(err)
	}
	inventoryBytes, _ := CanonicalInventoryBytes(inventory)
	binding, _ := BindFile("workspace/main/openapi-inventory.json", inventoryBytes, MaxInventoryBytes)
	expected := ExpectedResponse{StatusCode: 200, Body: DigestExpectation{SHA256: SHA256(nil), Bytes: 0}, Headers: []HeaderExpectation{}}
	for _, test := range []struct {
		name   string
		key    string
		target ReplayTarget
		path   string
		auth   *ReplayAuth
		want   string
	}{
		{name: "remote-target", key: "get /health", target: ReplayTarget{Scheme: "https", Host: "198.51.100.1", Port: 443, BasePath: "/"}, path: "/health", want: "loopback"},
		{name: "dns-loopback", key: "get /health", target: ReplayTarget{Scheme: "http", Host: "localhost", Port: 8080, BasePath: "/"}, path: "/health", want: "loopback"},
		{name: "write-method", key: "post /items/{id}", target: ReplayTarget{Scheme: "http", Host: "127.0.0.1", Port: 8080, BasePath: "/"}, path: "/items/1", auth: &ReplayAuth{Scheme: "apiKey", AuthRef: "STEAMAI_AUTH_KEY"}, want: "GET, HEAD, or OPTIONS"},
		{name: "raw-secret", key: "get /items/{id}", target: ReplayTarget{Scheme: "http", Host: "127.0.0.1", Port: 8080, BasePath: "/"}, path: "/items/1", auth: &ReplayAuth{Scheme: "bearer", AuthRef: "fixture-secret"}, want: "auth binding"},
		{name: "path-drift", key: "get /items/{id}", target: ReplayTarget{Scheme: "http", Host: "127.0.0.1", Port: 8080, BasePath: "/"}, path: "/other/1", auth: &ReplayAuth{Scheme: "bearer", AuthRef: "STEAMAI_AUTH_KEY"}, want: "does not match"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewReplayRequest(inventory, binding, test.key, test.target, test.path, test.auth, expected, DefaultReplayLimits())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want %q", err, test.want)
			}
		})
	}
}
