package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/client/vnet"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	VNetResourceName = "vnet"
)

var _ resource.Resource = &VNetResource{}
var _ resource.ResourceWithImportState = &VNetResource{}

func NewVNetResource() resource.Resource {
	return &VNetResource{}
}

type VNetResource struct {
	Data *KowabungaProviderData
}

type VNetResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Desc      types.String `tfsdk:"desc"`
	Zone      types.String `tfsdk:"zone"`
	VLAN      types.Int64  `tfsdk:"vlan"`
	Interface types.String `tfsdk:"interface"`
	Private   types.Bool   `tfsdk:"private"`
}

func (r *VNetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, VNetResourceName)
}

func (r *VNetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *VNetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *VNetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a virtual network resource",
		Attributes: map[string]schema.Attribute{
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyVLAN: schema.Int64Attribute{
				MarkdownDescription: "VLAN ID",
				Required:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
					int64validator.AtMost(4095),
				},
			},
			KeyInterface: schema.StringAttribute{
				MarkdownDescription: "Host bridge network interface",
				Required:            true,
			},
			KeyPrivate: schema.BoolAttribute{
				MarkdownDescription: "Whether the virtual network is private or public",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts virtual network from Terraform model to Kowabunga API model
func vnetResourceToModel(d *VNetResourceModel) models.VNet {
	return models.VNet{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Vlan:        d.VLAN.ValueInt64Pointer(),
		Interface:   d.Interface.ValueStringPointer(),
		Private:     d.Private.ValueBoolPointer(),
	}
}

// converts virtual network from Kowabunga API model to Terraform model
func vnetModelToResource(r *models.VNet, d *VNetResourceModel) {
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.VLAN = types.Int64PointerValue(r.Vlan)
	d.Interface = types.StringPointerValue(r.Interface)
	d.Private = types.BoolPointerValue(r.Private)
}

func (r *VNetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *VNetResourceModel
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

	// create a new virtual network
	cfg := vnetResourceToModel(data)
	params := zone.NewCreateVNetParams().WithZoneID(zoneId).WithBody(&cfg)
	obj, err := r.Data.K.Zone.CreateVNet(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	vnetModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created vnet resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VNetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *VNetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := vnet.NewGetVNetParams().WithVnetID(data.ID.ValueString())
	obj, err := r.Data.K.Vnet.GetVNet(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	vnetModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VNetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *VNetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := vnetResourceToModel(data)
	params := vnet.NewUpdateVNetParams().WithVnetID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Vnet.UpdateVNet(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VNetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *VNetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := vnet.NewDeleteVNetParams().WithVnetID(data.ID.ValueString())
	_, err := r.Data.K.Vnet.DeleteVNet(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
