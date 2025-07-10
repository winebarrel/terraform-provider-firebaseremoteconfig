package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/winebarrel/google-api-go-client/firebaseremoteconfig/v1"
)

var (
	_ resource.ResourceWithConfigure   = &Parameter{}
	_ resource.ResourceWithImportState = &Parameter{}
)

func NewParameter() resource.Resource {
	return &Parameter{}
}

type Parameter struct {
	client *FirebaseRemoteConfigClient
}

type ParameterModel struct {
	Project           types.String                   `tfsdk:"project"`
	Key               types.String                   `tfsdk:"key"`
	Description       types.String                   `tfsdk:"description"`
	ValueType         types.String                   `tfsdk:"value_type"`
	DefaultValue      *ParameterValueModel           `tfsdk:"default_value"`
	ConditionalValues map[string]ParameterValueModel `tfsdk:"conditional_values"`
}

type ParameterValueModel struct {
	UseInAppDefault types.Bool   `tfsdk:"use_in_app_default"`
	Value           types.String `tfsdk:"value"`
}

type ConditionalParameterValueModel struct {
	UseInAppDefault types.Bool   `tfsdk:"use_in_app_default"`
	Value           types.String `tfsdk:"value"`
}

func (r *Parameter) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parameter"
}

func (r *Parameter) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Optional: true,
			},
			"key": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
			},
			"value_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("STRING"),
				Validators: []validator.String{
					stringvalidator.OneOf(
						"PARAMETER_VALUE_TYPE_UNSPECIFIED",
						"STRING",
						"BOOLEAN",
						"NUMBER",
						"JSON",
					),
				},
			},
			"default_value": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"use_in_app_default": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(false),
					},
					"value": schema.StringAttribute{
						Required: true,
					},
				},
			},
			"conditional_values": schema.MapNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"use_in_app_default": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
						},
						"value": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
		},
	}
}

func (r *Parameter) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*FirebaseRemoteConfigClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *FirebaseRemoteConfigClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
	}

	r.client = client
}

func (r *Parameter) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ParameterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rc, err := r.client.GetRemoteConfig(plan.Project.ValueString()).Do()

	if err != nil {
		resp.Diagnostics.AddError("Error Getting Firebase Remote Config", err.Error())
		return
	}

	param := firebaseremoteconfig.RemoteConfigParameter{
		DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{
			Value: plan.DefaultValue.Value.ValueString(),
		},
	}

	if !plan.Description.IsNull() {
		param.Description = plan.Description.ValueString()
	}

	if !plan.ValueType.IsNull() {
		param.ValueType = plan.ValueType.ValueString()
	}

	if !plan.DefaultValue.UseInAppDefault.IsNull() {
		param.DefaultValue.UseInAppDefault = plan.DefaultValue.UseInAppDefault.ValueBool()
	}

	if len(plan.ConditionalValues) >= 1 {
		param.ConditionalValues = map[string]firebaseremoteconfig.RemoteConfigParameterValue{}

		for key, condVal := range plan.ConditionalValues {
			v := firebaseremoteconfig.RemoteConfigParameterValue{
				Value: condVal.Value.ValueString(),
			}

			if !condVal.UseInAppDefault.IsNull() {
				v.UseInAppDefault = condVal.UseInAppDefault.ValueBool()
			}

			param.ConditionalValues[key] = v
		}
	}

	rc.Parameters[plan.Key.ValueString()] = param
	_, err = r.client.UpdateRemoteConfig(plan.Project.ValueString(), rc).Do()

	if err != nil {
		resp.Diagnostics.AddError("Error Updating Firebase Remote Config", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *Parameter) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ParameterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rc, err := r.client.GetRemoteConfig(state.Project.ValueString()).Do()

	if err != nil {
		resp.Diagnostics.AddError("Error Getting Firebase Remote Config", err.Error())
		return
	}

	if param, ok := rc.Parameters[state.Key.ValueString()]; ok {

		state.Description = types.StringValue(param.Description)
		state.ValueType = types.StringValue(param.ValueType)

		state.DefaultValue = &ParameterValueModel{
			Value:           types.StringValue(param.DefaultValue.Value),
			UseInAppDefault: types.BoolValue(param.DefaultValue.UseInAppDefault),
		}

		if len(state.ConditionalValues) >= 1 {
			param.ConditionalValues = map[string]firebaseremoteconfig.RemoteConfigParameterValue{}

			for key, condVal := range state.ConditionalValues {
				v := firebaseremoteconfig.RemoteConfigParameterValue{
					Value: condVal.Value.ValueString(),
				}

				if !condVal.UseInAppDefault.IsNull() {
					v.UseInAppDefault = condVal.UseInAppDefault.ValueBool()
				}

				param.ConditionalValues[key] = v
			}
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	} else {
		resp.State.RemoveResource(ctx)
	}

	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *Parameter) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ParameterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rc, err := r.client.GetRemoteConfig(plan.Project.ValueString()).Do()

	if err != nil {
		resp.Diagnostics.AddError("Error Getting Firebase Remote Config", err.Error())
		return
	}

	param := firebaseremoteconfig.RemoteConfigParameter{
		DefaultValue: &firebaseremoteconfig.RemoteConfigParameterValue{
			Value: plan.DefaultValue.Value.ValueString(),
		},
	}

	if !plan.Description.IsNull() {
		param.Description = plan.Description.ValueString()
	}

	if !plan.ValueType.IsNull() {
		param.ValueType = plan.ValueType.ValueString()
	}

	if !plan.DefaultValue.UseInAppDefault.IsNull() {
		param.DefaultValue.UseInAppDefault = plan.DefaultValue.UseInAppDefault.ValueBool()
	}

	if len(plan.ConditionalValues) >= 1 {
		param.ConditionalValues = map[string]firebaseremoteconfig.RemoteConfigParameterValue{}

		for key, condVal := range plan.ConditionalValues {
			v := firebaseremoteconfig.RemoteConfigParameterValue{
				Value: condVal.Value.ValueString(),
			}

			if !condVal.UseInAppDefault.IsNull() {
				v.UseInAppDefault = condVal.UseInAppDefault.ValueBool()
			}

			param.ConditionalValues[key] = v
		}
	}

	rc.Parameters[plan.Key.ValueString()] = param
	_, err = r.client.UpdateRemoteConfig(plan.Project.ValueString(), rc).Do()

	if err != nil {
		resp.Diagnostics.AddError("Error Updating Firebase Remote Config", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)

	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *Parameter) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ParameterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rc, err := r.client.GetRemoteConfig(state.Project.ValueString()).Do()

	if err != nil {
		resp.Diagnostics.AddError("Error Getting Firebase Remote Config", err.Error())
		return
	}

	delete(rc.Parameters, state.Key.ValueString())
	_, err = r.client.UpdateRemoteConfig(state.Project.ValueString(), rc).Do()

	if err != nil {
		resp.Diagnostics.AddError("Error Updating Firebase Remote Config", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *Parameter) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
