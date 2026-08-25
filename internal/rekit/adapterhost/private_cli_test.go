package adapterhost

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmbeddedPrivateInvocationRecognition(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "parent", args: []string{privateAuthorizedVMPIDAFlag}, want: true},
		{name: "parent assigned", args: []string{privateAuthorizedVMPIDAFlag + "=true"}, want: true},
		{name: "child", args: []string{privateChildVMPIDAFlag}, want: true},
		{name: "ordinary", args: []string{"-prepare-vmp-ida-index-request"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := IsEmbeddedPrivateInvocation(test.args); got != test.want {
				t.Fatalf("IsEmbeddedPrivateInvocation(%q) = %t, want %t", test.args, got, test.want)
			}
		})
	}
}

func TestEmbeddedPrivateInvocationRejectsMixedOrMalformedModes(t *testing.T) {
	validBinding := `{"schemaVersion":1,"lane":"main","controlGeneration":0,"owner":{"lane":"main","currentExecutor":"executor-a","executorGeneration":1}}`
	for _, test := range []struct {
		name    string
		args    []string
		message string
	}{
		{
			name:    "both modes",
			args:    []string{privateAuthorizedVMPIDAFlag, privateChildVMPIDAFlag},
			message: "requires exactly one parent or child mode",
		},
		{
			name:    "parent with child flag",
			args:    []string{privateAuthorizedVMPIDAFlag, "-executor", "executor-a"},
			message: "parent flags cannot be combined",
		},
		{
			name:    "trailing binding JSON",
			args:    []string{privateAuthorizedVMPIDAFlag, "-execution-control-binding-json", validBinding + `{}`},
			message: "exactly one JSON object",
		},
		{
			name:    "unknown binding field",
			args:    []string{privateAuthorizedVMPIDAFlag, "-execution-control-binding-json", strings.TrimSuffix(validBinding, "}") + `,"extra":true}`},
			message: "unknown field",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			handled, code := RunEmbeddedPrivate(test.args, &stdout, &stderr)
			if !handled || code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.message) {
				t.Fatalf("handled=%t code=%d stdout=%q stderr=%q, want %q", handled, code, stdout.String(), stderr.String(), test.message)
			}
		})
	}
}
