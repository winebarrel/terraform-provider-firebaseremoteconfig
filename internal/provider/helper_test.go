package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/google-api-go-client/firebaseremoteconfig/v1"
	"github.com/winebarrel/google-api-go-client/option"
)

const testProject = "test-project"

// fakeServer is an in-memory fake of the Firebase Remote Config REST API.
type fakeServer struct {
	*httptest.Server

	mu      sync.Mutex
	configs map[string]*firebaseremoteconfig.RemoteConfig // keyed by project ID
	puts    []*http.Request
	// failWith, when non-zero, makes every request fail with this status code.
	failWith int
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()

	fs := &fakeServer{
		configs: map[string]*firebaseremoteconfig.RemoteConfig{},
	}

	fs.Server = httptest.NewServer(http.HandlerFunc(fs.handle))
	t.Cleanup(fs.Close)

	return fs
}

func (fs *fakeServer) handle(w http.ResponseWriter, r *http.Request) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.failWith != 0 {
		http.Error(w, `{"error":{"code":500,"message":"injected error"}}`, fs.failWith)
		return
	}

	// /v1/projects/{project}/remoteConfig
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if len(parts) != 4 || parts[0] != "v1" || parts[1] != "projects" || parts[3] != "remoteConfig" {
		http.NotFound(w, r)
		return
	}

	project := parts[2]

	switch r.Method {
	case http.MethodGet:
		rc, ok := fs.configs[project]

		if !ok {
			rc = &firebaseremoteconfig.RemoteConfig{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rc)
	case http.MethodPut:
		rc := &firebaseremoteconfig.RemoteConfig{}

		if err := json.NewDecoder(r.Body).Decode(rc); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fs.configs[project] = rc
		fs.puts = append(fs.puts, r)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rc)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (fs *fakeServer) setConfig(project string, rc *firebaseremoteconfig.RemoteConfig) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.configs[project] = rc
}

func (fs *fakeServer) config(project string) *firebaseremoteconfig.RemoteConfig {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.configs[project]
}

func (fs *fakeServer) putRequests() []*http.Request {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]*http.Request(nil), fs.puts...)
}

func (fs *fakeServer) setFailWith(status int) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.failWith = status
}

func newTestClient(t *testing.T, fs *fakeServer) *FirebaseRemoteConfigClient {
	t.Helper()

	service, err := firebaseremoteconfig.NewService(
		context.Background(),
		option.WithEndpoint(fs.URL+"/"),
		option.WithoutAuthentication(),
		option.WithHTTPClient(fs.Client()),
	)
	require.NoError(t, err)

	return &FirebaseRemoteConfigClient{
		service: service,
		project: testProject,
	}
}

func resourceSchema(t *testing.T, r resource.Resource) resource.SchemaResponse {
	t.Helper()

	resp := resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	return resp
}

func newPlan(t *testing.T, r resource.Resource, model any) tfsdk.Plan {
	t.Helper()

	plan := tfsdk.Plan{Schema: resourceSchema(t, r).Schema}
	diags := plan.Set(context.Background(), model)
	require.False(t, diags.HasError(), "%v", diags)

	return plan
}

func newState(t *testing.T, r resource.Resource, model any) tfsdk.State {
	t.Helper()

	state := tfsdk.State{Schema: resourceSchema(t, r).Schema}
	diags := state.Set(context.Background(), model)
	require.False(t, diags.HasError(), "%v", diags)

	return state
}

func emptyState(t *testing.T, r resource.Resource) tfsdk.State {
	t.Helper()

	s := resourceSchema(t, r).Schema
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}
}
