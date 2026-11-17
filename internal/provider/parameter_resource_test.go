package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/google-api-go-client/firebaseremoteconfig/v1"
)

func newTestParameter(t *testing.T, fs *fakeServer) *Parameter {
	t.Helper()

	r := &Parameter{}
	resp := resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: newTestClient(t, fs)}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	return r
}

func baseParameterModel() ParameterModel {
	return ParameterModel{
		Project:     types.StringNull(),
		Key:         types.StringValue("foo"),
		Description: types.StringValue("my param"),
		ValueType:   types.StringValue("JSON"),
		DefaultValue: &ParameterValueModel{
			UseInAppDefault: types.BoolValue(false),
			Value:           types.StringValue(`{"foo":"bar"}`),
		},
		ConditionalValues: map[string]ParameterValueModel{
			"ios": {
				UseInAppDefault: types.BoolValue(false),
				Value:           types.StringValue(`{"foo":"ios"}`),
			},
			"android": {
				UseInAppDefault: types.BoolValue(true),
				Value:           types.StringValue(""),
			},
		},
	}
}

func createParameter(t *testing.T, r *Parameter, model ParameterModel) ParameterModel {
	t.Helper()

	ctx := context.Background()
	resp := resource.CreateResponse{State: emptyState(t, r)}
	r.Create(ctx, resource.CreateRequest{Plan: newPlan(t, r, model)}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	var got ParameterModel
	require.False(t, resp.State.Get(ctx, &got).HasError())

	return got
}

func readParameter(t *testing.T, r *Parameter, model ParameterModel) (ParameterModel, bool) {
	t.Helper()

	ctx := context.Background()
	state := newState(t, r, model)
	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	if resp.State.Raw.IsNull() {
		return ParameterModel{}, false
	}

	var got ParameterModel
	require.False(t, resp.State.Get(ctx, &got).HasError())

	return got, true
}

func TestParameter_Metadata(t *testing.T) {
	resp := resource.MetadataResponse{}
	NewParameter().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "firebaseremoteconfig"}, &resp)
	assert.Equal(t, "firebaseremoteconfig_parameter", resp.TypeName)
}

func TestParameter_ConfigureNilProviderData(t *testing.T) {
	r := &Parameter{}
	resp := resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{}, &resp)

	assert.False(t, resp.Diagnostics.HasError())
	assert.Nil(t, r.client)
}

func TestParameter_ConfigureUnexpectedType(t *testing.T) {
	r := &Parameter{}
	resp := resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "invalid"}, &resp)

	assert.True(t, resp.Diagnostics.HasError())
}

func TestParameter_Create(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, &firebaseremoteconfig.RemoteConfig{
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"existing": {DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{Value: "keep"}},
		},
	})
	r := newTestParameter(t, fs)

	model := baseParameterModel()
	got := createParameter(t, r, model)
	assert.Equal(t, model, got)

	rc := fs.config(testProject)
	require.NotNil(t, rc)

	// Other parameters must be preserved.
	assert.Equal(t, "keep", rc.Parameters["existing"].DefaultValue.Value)

	param := rc.Parameters["foo"]
	assert.Equal(t, "my param", param.Description)
	assert.Equal(t, "JSON", param.ValueType)
	assert.Equal(t, `{"foo":"bar"}`, param.DefaultValue.Value)
	assert.False(t, param.DefaultValue.UseInAppDefault)
	assert.Equal(t, map[string]firebaseremoteconfig.RemoteConfigParameterValue{
		"ios":     {Value: `{"foo":"ios"}`},
		"android": {UseInAppDefault: true},
	}, param.ConditionalValues)
}

func TestParameter_CreateWithResourceProject(t *testing.T) {
	fs := newFakeServer(t)
	r := newTestParameter(t, fs)

	model := baseParameterModel()
	model.Project = types.StringValue("other-project")
	createParameter(t, r, model)

	assert.Nil(t, fs.config(testProject))
	require.NotNil(t, fs.config("other-project"))
	assert.Contains(t, fs.config("other-project").Parameters, "foo")
}

func TestParameter_CreateError(t *testing.T) {
	fs := newFakeServer(t)
	fs.setFailWith(http.StatusInternalServerError)
	r := newTestParameter(t, fs)

	resp := resource.CreateResponse{State: emptyState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{Plan: newPlan(t, r, baseParameterModel())}, &resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Error Getting Firebase Remote Config", resp.Diagnostics.Errors()[0].Summary())
}

func TestParameter_Read(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, &firebaseremoteconfig.RemoteConfig{
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"foo": {
				Description: "changed outside terraform",
				ValueType:   "STRING",
				DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{
					Value: "remote",
				},
				ConditionalValues: map[string]firebaseremoteconfig.RemoteConfigParameterValue{
					"ios": {UseInAppDefault: true},
				},
			},
		},
	})
	r := newTestParameter(t, fs)

	got, ok := readParameter(t, r, baseParameterModel())
	require.True(t, ok)

	assert.Equal(t, ParameterModel{
		Project:     types.StringNull(),
		Key:         types.StringValue("foo"),
		Description: types.StringValue("changed outside terraform"),
		ValueType:   types.StringValue("STRING"),
		DefaultValue: &ParameterValueModel{
			UseInAppDefault: types.BoolValue(false),
			Value:           types.StringValue("remote"),
		},
		ConditionalValues: map[string]ParameterValueModel{
			"ios": {
				UseInAppDefault: types.BoolValue(true),
				Value:           types.StringValue(""),
			},
		},
	}, got)
}

func TestParameter_ReadNotFound(t *testing.T) {
	fs := newFakeServer(t)
	r := newTestParameter(t, fs)

	_, ok := readParameter(t, r, baseParameterModel())
	assert.False(t, ok, "resource should be removed from state")
}

func TestParameter_ReadError(t *testing.T) {
	fs := newFakeServer(t)
	fs.setFailWith(http.StatusInternalServerError)
	r := newTestParameter(t, fs)

	state := newState(t, r, baseParameterModel())
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Error Getting Firebase Remote Config", resp.Diagnostics.Errors()[0].Summary())
}

func TestParameter_Update(t *testing.T) {
	fs := newFakeServer(t)
	r := newTestParameter(t, fs)
	ctx := context.Background()

	before := createParameter(t, r, baseParameterModel())

	after := baseParameterModel()
	after.Description = types.StringValue("updated")
	after.ValueType = types.StringValue("STRING")
	after.DefaultValue.Value = types.StringValue("new")
	after.ConditionalValues = nil

	resp := resource.UpdateResponse{State: newState(t, r, before)}
	r.Update(ctx, resource.UpdateRequest{
		Plan:  newPlan(t, r, after),
		State: newState(t, r, before),
	}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	var got ParameterModel
	require.False(t, resp.State.Get(ctx, &got).HasError())
	assert.Equal(t, after, got)

	param := fs.config(testProject).Parameters["foo"]
	assert.Equal(t, "updated", param.Description)
	assert.Equal(t, "STRING", param.ValueType)
	assert.Equal(t, "new", param.DefaultValue.Value)
	assert.Empty(t, param.ConditionalValues)
}

func TestParameter_Delete(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, &firebaseremoteconfig.RemoteConfig{
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"existing": {DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{Value: "keep"}},
		},
	})
	r := newTestParameter(t, fs)

	model := createParameter(t, r, baseParameterModel())
	require.Contains(t, fs.config(testProject).Parameters, "foo")

	state := newState(t, r, model)
	resp := resource.DeleteResponse{State: state}
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	assert.True(t, resp.State.Raw.IsNull())
	assert.NotContains(t, fs.config(testProject).Parameters, "foo")
	assert.Contains(t, fs.config(testProject).Parameters, "existing")
}

func TestParameter_ImportState(t *testing.T) {
	r := NewParameter().(*Parameter)
	ctx := context.Background()

	resp := resource.ImportStateResponse{State: emptyState(t, r)}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "foo"}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	var key types.String
	require.False(t, resp.State.GetAttribute(ctx, path.Root("key"), &key).HasError())
	assert.Equal(t, "foo", key.ValueString())
}

func TestParameter_CreateAndReadWithoutDefaultValue(t *testing.T) {
	fs := newFakeServer(t)
	r := newTestParameter(t, fs)

	model := baseParameterModel()
	model.DefaultValue = nil
	createParameter(t, r, model)

	got, ok := readParameter(t, r, model)
	require.True(t, ok)
	assert.Nil(t, got.DefaultValue)
	assert.Equal(t, model.ConditionalValues, got.ConditionalValues)
}

func TestParameter_ReadDetectsRemovedConditionalValues(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, &firebaseremoteconfig.RemoteConfig{
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"foo": {
				Description:  "my param",
				ValueType:    "JSON",
				DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{Value: `{"foo":"bar"}`},
			},
		},
	})
	r := newTestParameter(t, fs)

	got, ok := readParameter(t, r, baseParameterModel())
	require.True(t, ok)
	assert.Empty(t, got.ConditionalValues)
}
