package provider

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/winebarrel/google-api-go-client/firebaseremoteconfig/v1"
)

var (
	_ resource.ResourceWithConfigure   = &Conditions{}
	_ resource.ResourceWithImportState = &Conditions{}
)

func NewConditions() resource.Resource {
	return &Conditions{}
}

type Conditions struct {
	client *FirebaseRemoteConfigClient
}

type ConditionsModel struct {
	Project    types.String     `tfsdk:"project"`
	Conditions []ConditionModel `tfsdk:"conditions"`
}

type ConditionModel struct {
	Name       types.String `tfsdk:"name"`
	Expression types.String `tfsdk:"expression"`
	TagColor   types.String `tfsdk:"tag_color"`
}

func (r *Conditions) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_conditions"
}

func (r *Conditions) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the whole ordered list of Remote Config conditions. " +
			"The order of the list matters: when multiple conditions evaluate to true, the earlier one wins. " +
			"Only one `firebaseremoteconfig_conditions` resource should exist per project. " +
			"A condition referenced by parameters' `conditional_values` cannot be removed (or renamed): " +
			"remove the references and apply first, then remove the condition.",
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"conditions": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"expression": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"tag_color": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(""),
							Validators: []validator.String{
								stringvalidator.OneOf(
									"",
									"BLUE",
									"BROWN",
									"CYAN",
									"DEEP_ORANGE",
									"GREEN",
									"INDIGO",
									"LIME",
									"ORANGE",
									"PINK",
									"PURPLE",
									"TEAL",
								),
							},
						},
					},
				},
			},
		},
	}
}

func (r *Conditions) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (m *ConditionsModel) toAPI() []*firebaseremoteconfig.RemoteConfigCondition {
	conds := make([]*firebaseremoteconfig.RemoteConfigCondition, 0, len(m.Conditions))

	for _, c := range m.Conditions {
		conds = append(conds, &firebaseremoteconfig.RemoteConfigCondition{
			Name:       c.Name.ValueString(),
			Expression: c.Expression.ValueString(),
			TagColor:   c.TagColor.ValueString(),
		})
	}

	return conds
}

// conditionsReferencedError is returned when conditions to be removed are
// still referenced by parameters' conditional values.
type conditionsReferencedError struct {
	// refs maps a condition name to the parameter keys referencing it.
	refs map[string][]string
}

func (e *conditionsReferencedError) Error() string {
	var b strings.Builder

	b.WriteString("The following conditions cannot be removed because they are still referenced by parameters:\n")

	for _, name := range slices.Sorted(maps.Keys(e.refs)) {
		fmt.Fprintf(&b, "  - condition %q is referenced by parameters: %s\n", name, strings.Join(e.refs[name], ", "))
	}

	b.WriteString("\nRemove the conditions from the parameters' conditional_values and apply first, then remove the conditions.")

	return b.String()
}

// checkConditionReferences returns an error if any parameter has a conditional
// value for a condition that is not in conds.
func checkConditionReferences(params map[string]firebaseremoteconfig.RemoteConfigParameter, conds []*firebaseremoteconfig.RemoteConfigCondition) error {
	names := make(map[string]struct{}, len(conds))

	for _, c := range conds {
		names[c.Name] = struct{}{}
	}

	refs := map[string][]string{}

	for _, key := range slices.Sorted(maps.Keys(params)) {
		for _, condName := range slices.Sorted(maps.Keys(params[key].ConditionalValues)) {
			if _, ok := names[condName]; !ok {
				refs[condName] = append(refs[condName], key)
			}
		}
	}

	if len(refs) > 0 {
		return &conditionsReferencedError{refs: refs}
	}

	return nil
}

func addPutError(diags interface{ AddError(string, string) }, err error) {
	var refErr *conditionsReferencedError

	if errors.As(err, &refErr) {
		diags.AddError("Conditions Still Referenced", err.Error())
		return
	}

	diags.AddError("Error Updating Firebase Remote Config", err.Error())
}

func (r *Conditions) put(project string, conds []*firebaseremoteconfig.RemoteConfigCondition) error {
	r.client.mu.Lock()
	defer r.client.mu.Unlock()

	rc, err := r.client.GetRemoteConfig(project).Do()

	if err != nil {
		return fmt.Errorf("get remote config: %w", err)
	}

	if err := checkConditionReferences(rc.Parameters, conds); err != nil {
		return err
	}

	rc.Conditions = conds

	_, err = r.client.UpdateRemoteConfig(project, rc).Do()

	if err != nil {
		return fmt.Errorf("update remote config: %w", err)
	}

	return nil
}

func (r *Conditions) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ConditionsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.put(plan.Project.ValueString(), plan.toAPI()); err != nil {
		addPutError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *Conditions) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ConditionsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rc, err := r.client.GetRemoteConfig(state.Project.ValueString()).Do()

	if err != nil {
		resp.Diagnostics.AddError("Error Getting Firebase Remote Config", err.Error())
		return
	}

	state.Conditions = []ConditionModel{}

	for _, c := range rc.Conditions {
		tagColor := c.TagColor

		if tagColor == "CONDITION_DISPLAY_COLOR_UNSPECIFIED" {
			tagColor = ""
		}

		state.Conditions = append(state.Conditions, ConditionModel{
			Name:       types.StringValue(c.Name),
			Expression: types.StringValue(c.Expression),
			TagColor:   types.StringValue(tagColor),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Conditions) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ConditionsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.put(plan.Project.ValueString(), plan.toAPI()); err != nil {
		addPutError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *Conditions) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ConditionsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.put(state.Project.ValueString(), nil); err != nil {
		addPutError(&resp.Diagnostics, err)
		return
	}

	resp.State.RemoveResource(ctx)
}

// ImportState accepts a project ID as the import ID.
// If the ID equals the provider's project, `project` is left unset.
func (r *Conditions) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "" && (r.client == nil || req.ID != r.client.project) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), req.ID)...)
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("conditions"), []ConditionModel{})...)
}
