package websecurity

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReplayRequestBindingRequiresContentAddressedPath(t *testing.T) {
	data := []byte("canonical replay request bytes\n")
	binding, err := BindReplayRequest(data)
	if err != nil {
		t.Fatal(err)
	}
	expectedPath := ReplayRequestRoot + "/" + SHA256(data) + ".json"
	if binding.Path != expectedPath || binding.SHA256 != SHA256(data) || binding.Bytes != int64(len(data)) {
		t.Fatalf("content-addressed replay binding = %+v", binding)
	}
	if err := ValidateReplayRequestBinding(binding); err != nil {
		t.Fatal(err)
	}
	binding.Path = ReplayRequestRoot + "/request.json"
	if err := ValidateReplayRequestBinding(binding); err == nil {
		t.Fatal("accepted bounded replay request bytes under a non-content-addressed path")
	}
}

func TestExecuteReplayUsesExactLoopbackOnceAndReturnsRedactedDigestDiff(t *testing.T) {
	body := []byte("{\"id\":\"42\",\"ok\":true}\n")
	contentType := []byte("application/json")
	var requests atomic.Int32
	listener := listenLoopback(t)
	port := listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method != http.MethodGet || request.URL.Path != "/api/items/42" || request.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Errorf("unexpected replay request: method=%s path=%s auth=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
		}
		response.Header().Set("Content-Type", string(contentType))
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(body)
	})}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		<-serveDone
	})

	inventory, inventoryBinding, inventoryBytes := replayInventoryFixture(t, port)
	request := replayRequestFixture(t, inventory, inventoryBinding, port, body, contentType, DefaultReplayLimits())
	requestBytes, err := CanonicalReplayRequestBytes(request)
	if err != nil {
		t.Fatal(err)
	}
	requestBinding, err := BindReplayRequest(requestBytes)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ExecuteReplay(context.Background(), requestBinding, requestBytes, inventoryBytes, func(ref string) (string, bool) {
		return "fixture-secret", ref == "STEAMAI_AUTH_LOOPBACK_BEARER"
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "matched" || result.Delivery.Attempts != 1 || !result.Delivery.Certain || result.Actual == nil || result.Diff == nil || !result.Diff.Match || requests.Load() != 1 {
		t.Fatalf("unexpected bounded replay result: %+v requests=%d", result, requests.Load())
	}
	resultBytes, err := CanonicalReplayResultBytes(result)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(resultBytes, body) || bytes.Contains(resultBytes, []byte("fixture-secret")) || bytes.Contains(resultBytes, contentType) || !bytes.Contains(resultBytes, []byte(SHA256(body))) || !bytes.Contains(resultBytes, []byte(SHA256(contentType))) {
		t.Fatalf("bounded replay result leaked bytes or omitted digests:\n%s", resultBytes)
	}
	if _, err := DecodeReplayResult(resultBytes); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteReplayIgnoresAmbientProxyAndDoesNotRedirect(t *testing.T) {
	var targetRequests atomic.Int32
	var redirectRequests atomic.Int32
	targetListener := listenLoopback(t)
	targetPort := targetListener.Addr().(*net.TCPAddr).Port
	redirectListener := listenLoopback(t)
	redirectPort := redirectListener.Addr().(*net.TCPAddr).Port

	redirectServer := &http.Server{Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		redirectRequests.Add(1)
		response.WriteHeader(http.StatusTeapot)
	})}
	redirectDone := make(chan error, 1)
	go func() { redirectDone <- redirectServer.Serve(redirectListener) }()
	targetServer := &http.Server{Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		targetRequests.Add(1)
		response.Header().Set("Location", "http://127.0.0.1:"+strings.TrimSpace(intString(redirectPort))+"/redirected")
		response.WriteHeader(http.StatusFound)
	})}
	targetDone := make(chan error, 1)
	go func() { targetDone <- targetServer.Serve(targetListener) }()
	t.Cleanup(func() {
		_ = targetServer.Close()
		_ = redirectServer.Close()
		<-targetDone
		<-redirectDone
	})

	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	inventory, inventoryBinding, inventoryBytes := replayInventoryFixture(t, targetPort)
	expectedBody := []byte{}
	request := replayRequestFixture(t, inventory, inventoryBinding, targetPort, expectedBody, []byte{}, DefaultReplayLimits())
	request.Expected.StatusCode = http.StatusFound
	request.Expected.Headers = []HeaderExpectation{}
	requestBytes, _ := CanonicalReplayRequestBytes(request)
	requestBinding, _ := BindReplayRequest(requestBytes)
	result, err := ExecuteReplay(context.Background(), requestBinding, requestBytes, inventoryBytes, func(string) (string, bool) { return "fixture-secret", true })
	if err != nil {
		t.Fatal(err)
	}
	if targetRequests.Load() != 1 || redirectRequests.Load() != 0 || result.Delivery.Attempts != 1 || result.Status != "matched" {
		t.Fatalf("redirect/proxy boundary failed: result=%+v target=%d redirect=%d", result, targetRequests.Load(), redirectRequests.Load())
	}
}

func TestExecuteReplayMissingAuthFailsBeforeDelivery(t *testing.T) {
	var requests atomic.Int32
	listener := listenLoopback(t)
	port := listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.WriteHeader(http.StatusOK)
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close(); <-done })

	inventory, inventoryBinding, inventoryBytes := replayInventoryFixture(t, port)
	request := replayRequestFixture(t, inventory, inventoryBinding, port, nil, nil, DefaultReplayLimits())
	requestBytes, _ := CanonicalReplayRequestBytes(request)
	requestBinding, _ := BindReplayRequest(requestBytes)
	result, err := ExecuteReplay(context.Background(), requestBinding, requestBytes, inventoryBytes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed-before-delivery" || result.Delivery.Attempts != 0 || !result.Delivery.Certain || result.Delivery.ErrorCode != "auth-ref-unavailable" || requests.Load() != 0 {
		t.Fatalf("missing auth was not fail-closed before delivery: %+v requests=%d", result, requests.Load())
	}
}

func TestExecuteReplayConnectionFailureIsUncertainAndNeverRetried(t *testing.T) {
	listener := listenLoopback(t)
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	inventory, inventoryBinding, inventoryBytes := replayInventoryFixture(t, port)
	limits := DefaultReplayLimits()
	limits.RuntimeMillis = 250
	request := replayRequestFixture(t, inventory, inventoryBinding, port, nil, nil, limits)
	requestBytes, _ := CanonicalReplayRequestBytes(request)
	requestBinding, _ := BindReplayRequest(requestBytes)
	result, err := ExecuteReplay(context.Background(), requestBinding, requestBytes, inventoryBytes, func(string) (string, bool) { return "fixture-secret", true })
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "delivery-uncertain" || result.Delivery.Attempts != 1 || result.Delivery.Certain || result.Delivery.ErrorCode != "delivery-uncertain" {
		t.Fatalf("connection failure was not terminal uncertain delivery: %+v", result)
	}
}

func TestExecuteReplayResponseBodyLimitAbortsAfterDelivery(t *testing.T) {
	var requests atomic.Int32
	listener := listenLoopback(t)
	port := listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = response.Write(bytes.Repeat([]byte("x"), 64))
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close(); <-done })

	inventory, inventoryBinding, inventoryBytes := replayInventoryFixture(t, port)
	limits := DefaultReplayLimits()
	limits.ResponseBodyBytes = 8
	request := replayRequestFixture(t, inventory, inventoryBinding, port, nil, nil, limits)
	requestBytes, _ := CanonicalReplayRequestBytes(request)
	requestBinding, _ := BindReplayRequest(requestBytes)
	result, err := ExecuteReplay(context.Background(), requestBinding, requestBytes, inventoryBytes, func(string) (string, bool) { return "fixture-secret", true })
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "aborted-after-delivery" || result.Delivery.Attempts != 1 || !result.Delivery.Certain || result.Delivery.ErrorCode != "response-body-limit" || requests.Load() != 1 {
		t.Fatalf("body limit did not abort exact delivery: %+v requests=%d", result, requests.Load())
	}
}

func replayInventoryFixture(t *testing.T, port int) (Inventory, FileBinding, []byte) {
	t.Helper()
	openAPI := strings.Replace(syntheticOpenAPI, "18080", intString(port), 1)
	source, err := BindFile("inputs/openapi.json", []byte(openAPI), MaxOpenAPIBytes)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := ImportOpenAPI(source, []byte(openAPI))
	if err != nil {
		t.Fatal(err)
	}
	inventoryBytes, err := CanonicalInventoryBytes(inventory)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := BindFile("workspace/main/openapi-inventory.json", inventoryBytes, MaxInventoryBytes)
	if err != nil {
		t.Fatal(err)
	}
	return inventory, binding, inventoryBytes
}

func replayRequestFixture(t *testing.T, inventory Inventory, binding FileBinding, port int, body, contentType []byte, limits ReplayLimits) ReplayRequest {
	t.Helper()
	request, err := NewReplayRequest(
		inventory,
		binding,
		"get /items/{id}",
		ReplayTarget{Scheme: "http", Host: "127.0.0.1", Port: port, BasePath: "/api"},
		"/items/42",
		&ReplayAuth{Scheme: "bearer", AuthRef: "STEAMAI_AUTH_LOOPBACK_BEARER"},
		ExpectedResponse{
			StatusCode: http.StatusOK,
			Body:       DigestExpectation{SHA256: SHA256(body), Bytes: int64(len(body))},
			Headers:    []HeaderExpectation{{Name: "content-type", SHA256: SHA256(contentType), Bytes: int64(len(contentType))}},
		},
		limits,
	)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func listenLoopback(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listener
}

func intString(value int) string {
	return strconv.Itoa(value)
}
