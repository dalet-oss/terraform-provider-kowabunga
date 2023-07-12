package provider

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/dalet-oss/kowabunga-api/client"
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	ProviderName = "kowabunga"
	MimeJSON     = "application/json"
)

var _ provider.Provider = &KowabungaProvider{}

type KowabungaProviderModel struct {
	URI   types.String `tfsdk:"uri"`
	Token types.String `tfsdk:"token"`
}

type KowabungaProviderData struct {
	K     *client.Kowabunga
	Mutex *sync.Mutex
	Cond  *sync.Cond
}

type KowabungaProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string

	Data *KowabungaProviderData
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &KowabungaProvider{
			version: version,
		}
	}
}

func (p *KowabungaProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = ProviderName
	resp.Version = p.version
}

func (p *KowabungaProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			KeyURI: schema.StringAttribute{
				MarkdownDescription: "Kowabunga platform URI",
				Required:            true,
			},
			KeyToken: schema.StringAttribute{
				MarkdownDescription: "Kowabunga platform token (API key)",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
}

func newKowabungaClient(uri, token string) (*client.Kowabunga, error) {
	if uri == "" || token == "" {
		return nil, fmt.Errorf("The Kowabunga provider needs proper initialization parameters")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	r := httptransport.New(u.Host, client.DefaultBasePath, []string{u.Scheme})
	r.SetDebug(false)
	r.Consumers[MimeJSON] = runtime.JSONConsumer()
	r.Producers[MimeJSON] = runtime.JSONProducer()
	auths := []runtime.ClientAuthInfoWriter{
		httptransport.APIKeyAuth("x-token", "header", token),
	}
	r.DefaultAuthentication = httptransport.Compose(auths...)

	return client.New(r, strfmt.Default), nil
}

func (p *KowabungaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data KowabungaProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.URI.IsNull() || data.Token.IsNull() {
		resp.Diagnostics.AddError("Unknown Value", "An attribute value is not yet known")
		return
	}

	k, err := newKowabungaClient(data.URI.ValueString(), data.Token.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("No Kowabunga client", err.Error())
		return
	}

	var mut sync.Mutex
	var d = KowabungaProviderData{
		K:     k,
		Mutex: &mut,
		Cond:  sync.NewCond(&mut),
	}

	p.Data = &d
	resp.DataSourceData = &d
	resp.ResourceData = &d
}

func (p *KowabungaProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRegionResource,
		NewZoneResource,
		NewNetGWResource,
		NewHostResource,
		NewStoragePoolResource,
		NewTemplateResource,
		NewVolumeResource,
		NewVNetResource,
		NewSubnetResource,
		NewAdapterResource,
		NewProjectResource,
		NewInstanceResource,
		NewKceResource,
		NewDnsRecordResource,
	}
}

func (p *KowabungaProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
