package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func fakeDeveloperBroker(t *testing.T, endpoint, purpose string) {
	t.Helper()
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if r.Header.Get("Authorization") != "Bearer unit-broker" || json.NewDecoder(r.Body).Decode(&body) != nil || body["purpose"] != purpose || body["profile_ref"] != "andx-unit" {
			http.Error(w, "denied", 403)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"fields": map[string]string{"endpoint": endpoint, "developer_key": "andx_dev_" + strings.Repeat("k", 43)}})
	}))
	t.Cleanup(broker.Close)
	t.Setenv("HIMIND_CREDENTIAL_BROKER_URL", broker.URL+"/acquire")
	t.Setenv("HIMIND_CREDENTIAL_BROKER_TOKEN", "unit-broker")
}

func TestDeveloperPreviewUsesScopedBrokerAndReturnsExactRevision(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html><body>preview</body></html>`)
	pkg, err := packagePage(input{ProjectRoot: project, SourceRoot: "dist", OutputZip: "candidate.zip", PageKey: "exhibit-11", Version: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	hash := pkg["sha256"].(string)
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer andx_dev_"+strings.Repeat("k", 43) {
			t.Error("wrong authorization")
		}
		var data any
		switch r.URL.Path {
		case "/api/developer/projects/1/exhibits/11/pages":
			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Error(err)
			}
			if r.FormValue("expectedSha256") != hash || r.FormValue("tenantId") != "" {
				t.Error("unexpected upload fields")
			}
			data = map[string]any{"deployment": map[string]any{"id": "10", "tenantId": "1", "pageKey": "exhibit-11", "artifactSha256": hash}, "currentActivationId": "20", "reused": false}
		case "/api/developer/projects/1/exhibits/11/pages/10/health-check":
			data = map[string]any{"ok": true}
		case "/api/developer/projects/1/exhibits/11/pages/10/preview":
			data = map[string]any{"previewUrl": "https://example.com/api/interaction/preview#ticket", "liveSideEffects": false}
		default:
			t.Errorf("unexpected API %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data})
	}))
	defer service.Close()
	fakeDeveloperBroker(t, service.URL, "andx.page.deploy.preview")
	result, err := developerPreview(input{ProjectRoot: project, ArtifactZip: "candidate.zip", CredentialRef: "andx-unit", ProjectID: "1", ExhibitID: "11", ExpectedSHA256: hash})
	if err != nil {
		t.Fatal(err)
	}
	if result["expected_activation_id"] != "20" || result["ok"] != true {
		t.Fatalf("unexpected receipt: %v", result)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), strings.Repeat("k", 43)) {
		t.Fatal("receipt leaked secret")
	}
}

func TestDeveloperActivationRejectsConcurrentChangeWithoutRefreshingRevision(t *testing.T) {
	calls := 0
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/pages/10/activate") {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["expectedActivationId"] != "20" {
			t.Error("revision changed")
		}
		w.WriteHeader(409)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "CUSTOM_PAGE_ACTIVATION_INVALID", "message": "线上版本已变化", "data": nil})
	}))
	defer service.Close()
	fakeDeveloperBroker(t, service.URL, "andx.page.deploy.activate")
	_, err := developerActivate(input{ProjectRoot: t.TempDir(), CredentialRef: "andx-unit", ProjectID: "1", ExhibitID: "11", DeploymentID: "10", ExpectedActivationID: "20", ExpectedSHA256: strings.Repeat("a", 64), Reason: "确认"})
	if err == nil || calls != 1 {
		t.Fatalf("must fail once; calls=%d err=%v", calls, err)
	}
}

func TestDeveloperProfilesRejectMissingBindingAndInsecureService(t *testing.T) {
	if _, _, err := deliveryProfile(input{}, "andx.page.deploy.preview"); err == nil {
		t.Fatal("missing binding accepted")
	}
	fakeDeveloperBroker(t, "http://example.com", "andx.page.deploy.preview")
	if _, _, err := deliveryProfile(input{CredentialRef: "andx-unit", ProjectID: "1", ExhibitID: "11"}, "andx.page.deploy.preview"); err == nil {
		t.Fatal("insecure service accepted")
	}
}

func TestDeveloperAPIDoesNotFollowRedirects(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	response, err := doAPIRequest(t.Context(), http.DefaultClient, deployProfile{BaseURL: origin.URL, AdminToken: "unit-private"}, http.MethodPost, "/api/developer/test", []byte("{}"), "application/json")
	if err != nil || response.StatusCode != 307 || forwarded {
		t.Fatalf("redirect leaked request: status=%d err=%v forwarded=%t", response.StatusCode, err, forwarded)
	}
}

func TestDeveloperActivationValidatesArtifactAndOwnActivationReceipt(t *testing.T) {
	for _, invalid := range []string{"", "hash", "deployment", "activation", "tenant", "exhibit"} {
		t.Run(invalid, func(t *testing.T) {
			hash := strings.Repeat("a", 64)
			deployment := map[string]any{"id": "10", "tenantId": "1", "pageKey": "exhibit-11", "artifactSha256": hash}
			activation := map[string]any{"id": "21", "tenantId": "1", "exhibitId": "11", "deploymentId": "10", "scope": "exhibit", "action": "ACTIVATE"}
			switch invalid {
			case "hash":
				deployment["artifactSha256"] = strings.Repeat("b", 64)
			case "deployment":
				deployment["id"] = "99"
			case "activation":
				activation["id"] = "20"
			case "tenant":
				activation["tenantId"] = "2"
			case "exhibit":
				activation["exhibitId"] = "12"
			}
			service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"deployment": deployment, "activation": activation, "target": map[string]any{"project": map[string]string{"id": "1"}, "exhibit": map[string]string{"id": "11"}}}})
			}))
			defer service.Close()
			fakeDeveloperBroker(t, service.URL, "andx.page.deploy.activate")
			_, err := developerActivate(input{CredentialRef: "andx-unit", ProjectID: "1", ExhibitID: "11", DeploymentID: "10", ExpectedActivationID: "20", ExpectedSHA256: hash, Reason: "确认"})
			if (err != nil) != (invalid != "") {
				t.Fatalf("invalid=%q err=%v", invalid, err)
			}
		})
	}
}

func TestDeveloperEnrollmentKeepsBearerCodeOutOfModelReceipt(t *testing.T) {
	code := strings.Repeat("e", 32)
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"schema": "andx.device.enrollment.v1", "enrollmentToken": code, "exhibitId": "11", "expiresAt": "2026-11-25T13:00:00Z"}})
	}))
	defer service.Close()
	fakeDeveloperBroker(t, service.URL, "andx.page.device.enrollment")
	t.Setenv("HIMIND_PLUGIN_DATA_ROOT", t.TempDir())
	receipt, err := developerEnrollment(input{CredentialRef: "andx-unit", ProjectID: "1", ExhibitID: "11"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(receipt)
	if strings.Contains(string(encoded), code) {
		t.Fatal("enrollment bearer code leaked to ledger")
	}
	private, err := os.ReadFile(receipt["private_path"].(string))
	if err != nil || !strings.Contains(string(private), code) {
		t.Fatal("missing private device handoff")
	}
}
