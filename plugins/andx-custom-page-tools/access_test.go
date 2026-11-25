package main

import (
	"encoding/json"
	"himind-plugin/andx-custom-page-tools/internal/himindjsonrpc"
	"strings"
	"testing"
)

func TestAccessHandlerOwnsBusinessIdentityAndReturnsOnlyDeclaredPublicFields(t *testing.T) {
	for _, identity := range []string{"7", "8", ""} {
		body, _ := json.Marshal(map[string]any{"credential_id": "andx.developer", "document": map[string]string{"schema": "andx.developer.access.v1", "developer": "unit"}, "public_fields": map[string]string{"endpoint": "https://example.com"}, "response": map[string]any{"code": 0, "data": map[string]any{"developer": map[string]string{"id": identity, "handle": "unit"}}}})
		result, rpcError := handle(himindjsonrpc.Request{Method: "credential.import.verify", Params: body})
		if identity == "" {
			if rpcError == nil {
				t.Fatal("invalid identity accepted")
			}
			continue
		}
		if rpcError != nil {
			t.Fatal(rpcError.Message)
		}
		encoded, _ := json.Marshal(result)
		if strings.Contains(string(encoded), "developer_key") || strings.Contains(string(encoded), "andx_dev_") {
			t.Fatal("secret returned from identity handler")
		}
		object := result.(map[string]any)
		if object["identity"] != identity {
			t.Fatal("identity mismatch")
		}
	}
	missingCode := json.RawMessage(`{"credential_id":"andx.developer","document":{"schema":"andx.developer.access.v1"},"public_fields":{"endpoint":"https://example.com"},"response":{"data":{"developer":{"id":"7"}}}}`)
	if _, err := handle(himindjsonrpc.Request{Method: "credential.import.verify", Params: missingCode}); err == nil {
		t.Fatal("missing success envelope accepted")
	}
}
