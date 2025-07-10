package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/winebarrel/google-api-go-client/firebaseremoteconfig/v1"
	"github.com/winebarrel/google-api-go-client/option"
	"golang.org/x/oauth2/google"
)

var _ provider.Provider = &FirebaseRemoteConfigProvider{}

type FirebaseRemoteConfigProvider struct {
	version string
}

type FirebaseRemoteConfigProviderModel struct {
	Project types.String `tfsdk:"project"`
}

type FirebaseRemoteConfigClient struct {
	service *firebaseremoteconfig.Service
	project string
}

func (client *FirebaseRemoteConfigClient) GetRemoteConfig(resProj string) *firebaseremoteconfig.ProjectsGetRemoteConfigCall {
	proj := client.project
	if resProj != "" {
		proj = resProj
	}
	return client.service.Projects.GetRemoteConfig("projects/" + proj)
}

func (client *FirebaseRemoteConfigClient) UpdateRemoteConfig(resProj string, rc *firebaseremoteconfig.RemoteConfig) *firebaseremoteconfig.ProjectsUpdateRemoteConfigCall {
	proj := client.project
	if resProj != "" {
		proj = resProj
	}
	updateRemoteConfig := client.service.Projects.UpdateRemoteConfig("projects/"+proj, rc)
	updateRemoteConfig.Header().Add("If-Match", "*")
	return updateRemoteConfig
}

func (p *FirebaseRemoteConfigProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "firebaseremoteconfig"
	resp.Version = p.version
}

func (p *FirebaseRemoteConfigProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

func (p *FirebaseRemoteConfigProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var date FirebaseRemoteConfigProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &date)...)

	if resp.Diagnostics.HasError() {
		return
	}

	creds, err := google.FindDefaultCredentials(context.Background(), "https://www.googleapis.com/auth/firebase.remoteconfig")

	if err != nil {
		resp.Diagnostics.AddError("Unable to find default credentials", err.Error())
		return
	}

	service, err := firebaseremoteconfig.NewService(context.Background(), option.WithCredentials(creds), option.WithQuotaProject(date.Project.ValueString()))

	if err != nil {
		resp.Diagnostics.AddError("Unable to creates a new Service", err.Error())
		return
	}

	client := &FirebaseRemoteConfigClient{
		service: service,
		project: date.Project.ValueString(),
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *FirebaseRemoteConfigProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewParameter,
	}
}

func (p *FirebaseRemoteConfigProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// No Data Sources
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &FirebaseRemoteConfigProvider{
			version: version,
		}
	}
}
