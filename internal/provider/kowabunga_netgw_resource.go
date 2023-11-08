package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/netgw"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/zone"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	NetGWResourceName = "netgw"
)

var _ resource.Resource = &NetGWResource{}
var _ resource.ResourceWithImportState = &NetGWResource{}

func NewNetGWResource() resource.Resource {
	return &NetGWResource{}
}

type NetGWResource struct {
	Data *KowabungaProviderData
}

type NetGWResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Desc    types.String `tfsdk:"desc"`
	Zone    types.String `tfsdk:"zone"`
	Address types.String `tfsdk:"address"`
	Port    types.Int64  `tfsdk:"port"`
	Token   types.String `tfsdk:"token"`
}

func (r *NetGWResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, NetGWResourceName)
}

func (r *NetGWResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *NetGWResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *NetGWResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a netgw resource",
		Attributes: map[string]schema.Attribute{
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyAddress: schema.StringAttribute{
				MarkdownDescription: "Network gateway IPv4 address",
				Required:            true,
			},
			KeyPort: schema.Int64Attribute{
				MarkdownDescription: "Network gateway API port number",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
					int64validator.AtMost(65535),
				},
			},
			KeyToken: schema.StringAttribute{
				MarkdownDescription: "Network gateway API token",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts netgw from Terraform model to Kowabunga API model
func netgwResourceToModel(d *NetGWResourceModel) models.NetGW {
	return models.NetGW{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Address:     d.Address.ValueStringPointer(),
		Port:        d.Port.ValueInt64Pointer(),
		Token:       d.Token.ValueStringPointer(),
	}
}

// converts netgw from Kowabunga API model to Terraform model
func netgwModelToResource(r *models.NetGW, d *NetGWResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.Address = types.StringPointerValue(r.Address)
	d.Port = types.Int64PointerValue(r.Port)
	d.Token = types.StringPointerValue(r.Token)
}

func (r *NetGWResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *NetGWResourceModel
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

	// create a new network gateway
	cfg := netgwResourceToModel(data)
	params := zone.NewCreateNetGWParams().WithZoneID(zoneId).WithBody(&cfg)
	obj, err := r.Data.K.Zone.CreateNetGW(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	netgwModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created netgw resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetGWResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *NetGWResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := netgw.NewGetNetGWParams().WithNetgwID(data.ID.ValueString())
	obj, err := r.Data.K.Netgw.GetNetGW(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	netgwModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetGWResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *NetGWResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := netgwResourceToModel(data)
	params := netgw.NewUpdateNetGWParams().WithNetgwID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Netgw.UpdateNetGW(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetGWResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *NetGWResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := netgw.NewDeleteNetGWParams().WithNetgwID(data.ID.ValueString())
	_, err := r.Data.K.Netgw.DeleteNetGW(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
