package provider

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/winebarrel/google-api-go-client/firebaseremoteconfig/v1"
)

func newTestConditions(t *testing.T, fs *fakeServer) *Conditions {
	t.Helper()

	r := &Conditions{}
	resp := resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: newTestClient(t, fs)}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	return r
}

func baseConditionsModel() ConditionsModel {
	return ConditionsModel{
		Project: types.StringNull(),
		Conditions: []ConditionModel{
			{
				Name:       types.StringValue("ios"),
				Expression: types.StringValue("device.os == 'ios'"),
				TagColor:   types.StringValue("BLUE"),
			},
			{
				Name:       types.StringValue("android"),
				Expression: types.StringValue("device.os == 'android'"),
				TagColor:   types.StringValue(""),
			},
		},
	}
}

func createConditions(t *testing.T, r *Conditions, model ConditionsModel) ConditionsModel {
	t.Helper()

	ctx := context.Background()
	resp := resource.CreateResponse{State: emptyState(t, r)}
	r.Create(ctx, resource.CreateRequest{Plan: newPlan(t, r, model)}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	var got ConditionsModel
	require.False(t, resp.State.Get(ctx, &got).HasError())

	return got
}

func readConditions(t *testing.T, r *Conditions, model ConditionsModel) ConditionsModel {
	t.Helper()

	ctx := context.Background()
	state := newState(t, r, model)
	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	var got ConditionsModel
	require.False(t, resp.State.Get(ctx, &got).HasError())

	return got
}

func TestConditions_Metadata(t *testing.T) {
	resp := resource.MetadataResponse{}
	NewConditions().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "firebaseremoteconfig"}, &resp)
	assert.Equal(t, "firebaseremoteconfig_conditions", resp.TypeName)
}

func TestConditions_ConfigureNilProviderData(t *testing.T) {
	r := &Conditions{}
	resp := resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{}, &resp)

	assert.False(t, resp.Diagnostics.HasError())
	assert.Nil(t, r.client)
}

func TestConditions_ConfigureUnexpectedType(t *testing.T) {
	r := &Conditions{}
	resp := resource.ConfigureResponse{}
	r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "invalid"}, &resp)

	assert.True(t, resp.Diagnostics.HasError())
}

func TestConditions_Create(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, &firebaseremoteconfig.RemoteConfig{
		Conditions: []*firebaseremoteconfig.RemoteConfigCondition{
			{Name: "old", Expression: "true"},
		},
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"existing": {DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{Value: "keep"}},
		},
	})
	r := newTestConditions(t, fs)

	model := baseConditionsModel()
	got := createConditions(t, r, model)
	assert.Equal(t, model, got)

	rc := fs.config(testProject)
	require.NotNil(t, rc)

	// The whole list is replaced, keeping the configured order.
	assert.Equal(t, []*firebaseremoteconfig.RemoteConfigCondition{
		{Name: "ios", Expression: "device.os == 'ios'", TagColor: "BLUE"},
		{Name: "android", Expression: "device.os == 'android'"},
	}, rc.Conditions)

	// Parameters must be preserved.
	assert.Equal(t, "keep", rc.Parameters["existing"].DefaultValue.Value)
}

func TestConditions_CreateWithResourceProject(t *testing.T) {
	fs := newFakeServer(t)
	r := newTestConditions(t, fs)

	model := baseConditionsModel()
	model.Project = types.StringValue("other-project")
	createConditions(t, r, model)

	assert.Nil(t, fs.config(testProject))
	require.NotNil(t, fs.config("other-project"))
	assert.Len(t, fs.config("other-project").Conditions, 2)
}

func TestConditions_CreateError(t *testing.T) {
	fs := newFakeServer(t)
	fs.setFailWith(http.StatusInternalServerError)
	r := newTestConditions(t, fs)

	resp := resource.CreateResponse{State: emptyState(t, r)}
	r.Create(context.Background(), resource.CreateRequest{Plan: newPlan(t, r, baseConditionsModel())}, &resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Error Updating Firebase Remote Config", resp.Diagnostics.Errors()[0].Summary())
}

func TestConditions_Read(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, &firebaseremoteconfig.RemoteConfig{
		Conditions: []*firebaseremoteconfig.RemoteConfigCondition{
			{Name: "android", Expression: "device.os == 'android'", TagColor: "GREEN"},
			{Name: "ios", Expression: "device.os == 'ios'", TagColor: "CONDITION_DISPLAY_COLOR_UNSPECIFIED"},
		},
	})
	r := newTestConditions(t, fs)

	got := readConditions(t, r, baseConditionsModel())

	// Order follows the remote, and the unspecified color is normalized to "".
	assert.Equal(t, ConditionsModel{
		Project: types.StringNull(),
		Conditions: []ConditionModel{
			{
				Name:       types.StringValue("android"),
				Expression: types.StringValue("device.os == 'android'"),
				TagColor:   types.StringValue("GREEN"),
			},
			{
				Name:       types.StringValue("ios"),
				Expression: types.StringValue("device.os == 'ios'"),
				TagColor:   types.StringValue(""),
			},
		},
	}, got)
}

func TestConditions_ReadEmpty(t *testing.T) {
	fs := newFakeServer(t)
	r := newTestConditions(t, fs)

	got := readConditions(t, r, baseConditionsModel())

	// The resource stays in state with an empty list so that the drift is planned.
	assert.NotNil(t, got.Conditions)
	assert.Empty(t, got.Conditions)
}

func TestConditions_ReadError(t *testing.T) {
	fs := newFakeServer(t)
	fs.setFailWith(http.StatusInternalServerError)
	r := newTestConditions(t, fs)

	state := newState(t, r, baseConditionsModel())
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Error Getting Firebase Remote Config", resp.Diagnostics.Errors()[0].Summary())
}

func TestConditions_Update(t *testing.T) {
	fs := newFakeServer(t)
	r := newTestConditions(t, fs)
	ctx := context.Background()

	before := createConditions(t, r, baseConditionsModel())

	// Reorder, change and add conditions.
	after := ConditionsModel{
		Project: types.StringNull(),
		Conditions: []ConditionModel{
			{
				Name:       types.StringValue("web"),
				Expression: types.StringValue("app.id == 'web'"),
				TagColor:   types.StringValue("TEAL"),
			},
			{
				Name:       types.StringValue("android"),
				Expression: types.StringValue("device.os == 'android'"),
				TagColor:   types.StringValue("GREEN"),
			},
			{
				Name:       types.StringValue("ios"),
				Expression: types.StringValue("device.os == 'ios'"),
				TagColor:   types.StringValue("BLUE"),
			},
		},
	}

	resp := resource.UpdateResponse{State: newState(t, r, before)}
	r.Update(ctx, resource.UpdateRequest{
		Plan:  newPlan(t, r, after),
		State: newState(t, r, before),
	}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	var got ConditionsModel
	require.False(t, resp.State.Get(ctx, &got).HasError())
	assert.Equal(t, after, got)

	assert.Equal(t, []*firebaseremoteconfig.RemoteConfigCondition{
		{Name: "web", Expression: "app.id == 'web'", TagColor: "TEAL"},
		{Name: "android", Expression: "device.os == 'android'", TagColor: "GREEN"},
		{Name: "ios", Expression: "device.os == 'ios'", TagColor: "BLUE"},
	}, fs.config(testProject).Conditions)
}

func TestConditions_Delete(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, &firebaseremoteconfig.RemoteConfig{
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"existing": {DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{Value: "keep"}},
		},
	})
	r := newTestConditions(t, fs)

	model := createConditions(t, r, baseConditionsModel())
	require.Len(t, fs.config(testProject).Conditions, 2)

	state := newState(t, r, model)
	resp := resource.DeleteResponse{State: state}
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	assert.True(t, resp.State.Raw.IsNull())
	assert.Empty(t, fs.config(testProject).Conditions)
	assert.Contains(t, fs.config(testProject).Parameters, "existing")
}

func TestConditions_ImportState(t *testing.T) {
	fs := newFakeServer(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		id      string
		project types.String
	}{
		{name: "provider project", id: testProject, project: types.StringNull()},
		{name: "other project", id: "other-project", project: types.StringValue("other-project")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestConditions(t, fs)

			resp := resource.ImportStateResponse{State: emptyState(t, r)}
			r.ImportState(ctx, resource.ImportStateRequest{ID: tt.id}, &resp)
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

			var project types.String
			require.False(t, resp.State.GetAttribute(ctx, path.Root("project"), &project).HasError())
			assert.Equal(t, tt.project, project)

			var conds []ConditionModel
			require.False(t, resp.State.GetAttribute(ctx, path.Root("conditions"), &conds).HasError())
			assert.NotNil(t, conds)
			assert.Empty(t, conds)
		})
	}
}

// Terraform applies resources in parallel. Concurrent read-modify-write of the
// remote config must not lose each other's changes.
func TestConcurrentApplyDoesNotLoseUpdates(t *testing.T) {
	fs := newFakeServer(t)
	client := newTestClient(t, fs)
	ctx := context.Background()

	cr := &Conditions{client: client}
	condModel := baseConditionsModel()
	condPlan := newPlan(t, cr, condModel)

	const numParams = 20

	pr := &Parameter{client: client}
	paramPlans := make([]resource.CreateRequest, numParams)

	for i := range numParams {
		m := baseParameterModel()
		m.Key = types.StringValue(fmt.Sprintf("param%d", i))
		paramPlans[i] = resource.CreateRequest{Plan: newPlan(t, pr, m)}
	}

	condState := emptyState(t, cr)
	paramState := emptyState(t, pr)

	var wg sync.WaitGroup
	errs := make(chan error, numParams+1)

	wg.Go(func() {
		resp := resource.CreateResponse{State: condState}
		cr.Create(ctx, resource.CreateRequest{Plan: condPlan}, &resp)

		if resp.Diagnostics.HasError() {
			errs <- fmt.Errorf("conditions: %v", resp.Diagnostics)
		}
	})

	for i := range numParams {
		wg.Go(func() {
			resp := resource.CreateResponse{State: paramState}
			pr.Create(ctx, paramPlans[i], &resp)

			if resp.Diagnostics.HasError() {
				errs <- fmt.Errorf("param%d: %v", i, resp.Diagnostics)
			}
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	rc := fs.config(testProject)
	require.NotNil(t, rc)
	assert.Len(t, rc.Conditions, 2)
	assert.Len(t, rc.Parameters, numParams)
}

func TestConditionsSchema_ProjectRequiresReplace(t *testing.T) {
	s := resourceSchema(t, NewConditions()).Schema

	attr, ok := s.Attributes["project"].(schema.StringAttribute)
	require.True(t, ok)
	require.Len(t, attr.PlanModifiers, 1)

	resp := &planmodifier.StringResponse{}
	attr.PlanModifiers[0].PlanModifyString(context.Background(), planmodifier.StringRequest{
		StateValue: types.StringValue("a"),
		PlanValue:  types.StringValue("b"),
		State:      newState(t, NewConditions(), baseConditionsModel()),
		Plan:       newPlan(t, NewConditions(), baseConditionsModel()),
	}, resp)
	assert.True(t, resp.RequiresReplace)
}

func referencedConfig() *firebaseremoteconfig.RemoteConfig {
	return &firebaseremoteconfig.RemoteConfig{
		Conditions: []*firebaseremoteconfig.RemoteConfigCondition{
			{Name: "ios", Expression: "device.os == 'ios'", TagColor: "BLUE"},
			{Name: "android", Expression: "device.os == 'android'"},
		},
		Parameters: map[string]firebaseremoteconfig.RemoteConfigParameter{
			"bar": {
				DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{Value: "default"},
				ConditionalValues: map[string]firebaseremoteconfig.RemoteConfigParameterValue{
					"ios":     {Value: "for ios"},
					"android": {Value: "for android"},
				},
			},
			"baz": {
				ConditionalValues: map[string]firebaseremoteconfig.RemoteConfigParameterValue{
					"ios": {Value: "for ios"},
				},
			},
		},
	}
}

func TestConditions_UpdateRemovingReferencedConditionFails(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, referencedConfig())
	r := newTestConditions(t, fs)
	ctx := context.Background()

	before := baseConditionsModel()
	after := baseConditionsModel()
	after.Conditions = after.Conditions[1:] // remove "ios"

	resp := resource.UpdateResponse{State: newState(t, r, before)}
	r.Update(ctx, resource.UpdateRequest{
		Plan:  newPlan(t, r, after),
		State: newState(t, r, before),
	}, &resp)

	require.True(t, resp.Diagnostics.HasError())
	d := resp.Diagnostics.Errors()[0]
	assert.Equal(t, "Conditions Still Referenced", d.Summary())
	assert.Contains(t, d.Detail(), `condition "ios" is referenced by parameters: bar, baz`)
	assert.NotContains(t, d.Detail(), `"android"`)

	// Nothing is written.
	assert.Empty(t, fs.putRequests())
	assert.Equal(t, referencedConfig(), fs.config(testProject))
}

func TestConditions_UpdateRemovingUnreferencedCondition(t *testing.T) {
	rc := referencedConfig()
	delete(rc.Parameters["bar"].ConditionalValues, "android")
	fs := newFakeServer(t)
	fs.setConfig(testProject, rc)
	r := newTestConditions(t, fs)
	ctx := context.Background()

	before := baseConditionsModel()
	after := baseConditionsModel()
	after.Conditions = after.Conditions[:1] // remove "android"

	resp := resource.UpdateResponse{State: newState(t, r, before)}
	r.Update(ctx, resource.UpdateRequest{
		Plan:  newPlan(t, r, after),
		State: newState(t, r, before),
	}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	assert.Equal(t, []*firebaseremoteconfig.RemoteConfigCondition{
		{Name: "ios", Expression: "device.os == 'ios'", TagColor: "BLUE"},
	}, fs.config(testProject).Conditions)
	// Conditional values are never touched.
	assert.Equal(t, rc.Parameters, fs.config(testProject).Parameters)
}

func TestConditions_DeleteReferencedConditionsFails(t *testing.T) {
	fs := newFakeServer(t)
	fs.setConfig(testProject, referencedConfig())
	r := newTestConditions(t, fs)

	state := newState(t, r, baseConditionsModel())
	resp := resource.DeleteResponse{State: state}
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)

	require.True(t, resp.Diagnostics.HasError())
	d := resp.Diagnostics.Errors()[0]
	assert.Equal(t, "Conditions Still Referenced", d.Summary())
	assert.Contains(t, d.Detail(), `condition "android" is referenced by parameters: bar`)
	assert.Contains(t, d.Detail(), `condition "ios" is referenced by parameters: bar, baz`)

	// State is kept and nothing is written.
	assert.False(t, resp.State.Raw.IsNull())
	assert.Empty(t, fs.putRequests())
	assert.Equal(t, referencedConfig(), fs.config(testProject))
}

func TestCheckConditionReferences(t *testing.T) {
	params := referencedConfig().Parameters

	tests := []struct {
		name  string
		conds []*firebaseremoteconfig.RemoteConfigCondition
		refs  map[string][]string
	}{
		{
			name:  "all referenced conditions exist",
			conds: referencedConfig().Conditions,
		},
		{
			name:  "extra conditions are fine",
			conds: append(referencedConfig().Conditions, &firebaseremoteconfig.RemoteConfigCondition{Name: "web"}),
		},
		{
			name:  "missing ios",
			conds: referencedConfig().Conditions[1:],
			refs:  map[string][]string{"ios": {"bar", "baz"}},
		},
		{
			name: "no conditions",
			refs: map[string][]string{"ios": {"bar", "baz"}, "android": {"bar"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkConditionReferences(params, tt.conds)

			if tt.refs == nil {
				assert.NoError(t, err)
				return
			}

			var refErr *conditionsReferencedError
			require.ErrorAs(t, err, &refErr)
			assert.Equal(t, tt.refs, refErr.refs)
		})
	}
}
