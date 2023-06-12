package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/client/host"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	HostResourceName = "host"
)

var _ resource.Resource = &HostResource{}
var _ resource.ResourceWithImportState = &HostResource{}

func NewHostResource() resource.Resource {
	return &HostResource{}
}

type HostResource struct {
	Data *KowabungaProviderData
}

type HostResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Desc     types.String `tfsdk:"desc"`
	Zone     types.String `tfsdk:"zone"`
	Protocol types.String `tfsdk:"protocol"`
	Address  types.String `tfsdk:"address"`
	Port     types.Int64  `tfsdk:"port"`
	TlsKey   types.String `tfsdk:"key"`
	TlsCert  types.String `tfsdk:"cert"`
	TlsCA    types.String `tfsdk:"ca"`
	Price    types.Int64  `tfsdk:"price"`
	Currency types.String `tfsdk:"currency"`
}

func (r *HostResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, HostResourceName)
}

func (r *HostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *HostResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *HostResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a host resource",
		Attributes: map[string]schema.Attribute{
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyProtocol: schema.StringAttribute{
				MarkdownDescription: "libvirt host API access protocol",
				Required:            true,
			},
			KeyAddress: schema.StringAttribute{
				MarkdownDescription: "libvirt host API IPv4 address",
				Required:            true,
			},
			KeyPort: schema.Int64Attribute{
				MarkdownDescription: "libvirt host API port number",
				Computed:            true,
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
					int64validator.AtMost(65535),
				},
				Default: int64default.StaticInt64(0),
			},
			KeyTlsKey: schema.StringAttribute{
				MarkdownDescription: "libvirt host API TLS private key",
				Optional:            true,
				Sensitive:           true,
			},
			KeyTlsCert: schema.StringAttribute{
				MarkdownDescription: "libvirt host API TLS certificate",
				Optional:            true,
			},
			KeyTlsCA: schema.StringAttribute{
				MarkdownDescription: "libvirt host API TLS CA",
				Optional:            true,
			},
			KeyPrice: schema.Int64Attribute{
				MarkdownDescription: "libvirt host monthly price value",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyCurrency: schema.StringAttribute{
				MarkdownDescription: "libvirt host monthly price currency",
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString("EUR"),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts host from Terraform model to Kowabunga API model
func hostResourceToModel(d *HostResourceModel) models.Host {
	cost := models.Cost{
		Price:    d.Price.ValueInt64Pointer(),
		Currency: d.Currency.ValueStringPointer(),
	}
	hc := models.Host{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Protocol:    d.Protocol.ValueStringPointer(),
		Address:     d.Address.ValueStringPointer(),
		Port:        d.Port.ValueInt64(),
		Cost:        &cost,
	}

	if *hc.Protocol == models.HostProtocolTLS {
		tls := models.HostTLS{
			Key:  d.TlsKey.ValueStringPointer(),
			Cert: d.TlsCert.ValueStringPointer(),
			Ca:   d.TlsCA.ValueStringPointer(),
		}
		hc.TLS = &tls
	}

	return hc
}

// converts host from Kowabunga API model to Terraform model
func hostModelToResource(r *models.Host, d *HostResourceModel) {
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.Protocol = types.StringPointerValue(r.Protocol)
	d.Address = types.StringPointerValue(r.Address)
	d.Port = types.Int64Value(r.Port)
	if r.Cost != nil {
		d.Price = types.Int64PointerValue(r.Cost.Price)
		d.Currency = types.StringPointerValue(r.Cost.Currency)
	}
	if r.TLS != nil {
		d.TlsKey = types.StringPointerValue(r.TLS.Key)
		d.TlsCert = types.StringPointerValue(r.TLS.Cert)
		d.TlsCA = types.StringPointerValue(r.TLS.Ca)
	}
}

func (r *HostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *HostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	// find parent zone
	zoneId, err := getZoneID(r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	// create a new host
	cfg := hostResourceToModel(data)
	params := zone.NewCreateHostParams().WithZoneID(zoneId).WithBody(&cfg)
	obj, err := r.Data.K.Zone.CreateHost(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	tflog.Trace(ctx, "created host resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *HostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := host.NewGetHostParams().WithHostID(data.ID.ValueString())
	obj, err := r.Data.K.Host.GetHost(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	hostModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *HostResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := hostResourceToModel(data)
	params := host.NewUpdateHostParams().WithHostID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Host.UpdateHost(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *HostResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := host.NewDeleteHostParams().WithHostID(data.ID.ValueString())
	_, err := r.Data.K.Host.DeleteHost(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
