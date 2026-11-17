package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/google-api-go-client/firebaseremoteconfig/v1"
)

func TestProvider_Metadata(t *testing.T) {
	p := New("1.2.3")()
	resp := provider.MetadataResponse{}
	p.Metadata(context.Background(), provider.MetadataRequest{}, &resp)

	assert.Equal(t, "firebaseremoteconfig", resp.TypeName)
	assert.Equal(t, "1.2.3", resp.Version)
}

func TestProvider_Schema(t *testing.T) {
	p := New("test")()
	resp := provider.SchemaResponse{}
	p.Schema(context.Background(), provider.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	diags := resp.Schema.ValidateImplementation(context.Background())
	require.False(t, diags.HasError(), "%v", diags)

	attr, ok := resp.Schema.Attributes["project"]
	require.True(t, ok)
	assert.True(t, attr.IsRequired())
}

func TestProvider_Resources(t *testing.T) {
	p := New("test")()
	ctx := context.Background()

	var typeNames []string

	for _, f := range p.Resources(ctx) {
		r := f()

		resp := resource.MetadataResponse{}
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "firebaseremoteconfig"}, &resp)
		typeNames = append(typeNames, resp.TypeName)

		schemaResp := resource.SchemaResponse{}
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
		require.False(t, schemaResp.Diagnostics.HasError(), "%v", schemaResp.Diagnostics)

		diags := schemaResp.Schema.ValidateImplementation(ctx)
		require.False(t, diags.HasError(), "%s: %v", resp.TypeName, diags)
	}

	assert.ElementsMatch(t, []string{"firebaseremoteconfig_parameter", "firebaseremoteconfig_conditions"}, typeNames)
	assert.Empty(t, p.DataSources(ctx))
}

func TestProvider_Server(t *testing.T) {
	_, err := providerserver.NewProtocol6WithError(New("test")())()
	assert.NoError(t, err)
}

func TestClient_DefaultProject(t *testing.T) {
	fs := newFakeServer(t)
	client := newTestClient(t, fs)

	_, err := client.UpdateRemoteConfig("", &firebaseremoteconfig.RemoteConfig{
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"foo": {DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{Value: "bar"}},
		},
	}).Do()
	require.NoError(t, err)

	rc, err := client.GetRemoteConfig("").Do()
	require.NoError(t, err)
	assert.Equal(t, "bar", rc.Parameters["foo"].DefaultValue.Value)

	assert.NotNil(t, fs.config(testProject))
}

func TestClient_ResourceProjectOverridesProviderProject(t *testing.T) {
	fs := newFakeServer(t)
	client := newTestClient(t, fs)

	_, err := client.UpdateRemoteConfig("other-project", &firebaseremoteconfig.RemoteConfig{
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"foo": {DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{Value: "bar"}},
		},
	}).Do()
	require.NoError(t, err)

	assert.Nil(t, fs.config(testProject))
	require.NotNil(t, fs.config("other-project"))
	assert.Contains(t, fs.config("other-project").Parameters, "foo")
}

func TestClient_UpdateSendsIfMatchWildcard(t *testing.T) {
	fs := newFakeServer(t)
	client := newTestClient(t, fs)

	_, err := client.UpdateRemoteConfig("", &firebaseremoteconfig.RemoteConfig{}).Do()
	require.NoError(t, err)

	puts := fs.putRequests()
	require.Len(t, puts, 1)
	assert.Equal(t, "*", puts[0].Header.Get("If-Match"))
}
