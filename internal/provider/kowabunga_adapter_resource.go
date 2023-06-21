package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/client/adapter"
	"github.com/dalet-oss/kowabunga-api/client/subnet"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	AdapterResourceName = "adapter"
)

var _ resource.Resource = &AdapterResource{}
var _ resource.ResourceWithImportState = &AdapterResource{}

func NewAdapterResource() resource.Resource {
	return &AdapterResource{}
}

type AdapterResource struct {
	Data *KowabungaProviderData
}

type AdapterResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Desc      types.String `tfsdk:"desc"`
	Subnet    types.String `tfsdk:"subnet"`
	MAC       types.String `tfsdk:"hwaddress"`
	Addresses types.List   `tfsdk:"addresses"`
	Assign    types.Bool   `tfsdk:"assign"`
	Reserved  types.Bool   `tfsdk:"reserved"`
}

func (r *AdapterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, AdapterResourceName)
}

func (r *AdapterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *AdapterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *AdapterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a network adapter resource",
		Attributes: map[string]schema.Attribute{
			KeySubnet: schema.StringAttribute{
				MarkdownDescription: "Associated subnet name or ID",
				Required:            true,
			},
			KeyMAC: schema.StringAttribute{
				MarkdownDescription: "Network adapter hardware MAC address (e.g. 00:11:22:33:44:55). AUto-generated if unspecified.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyAddresses: schema.ListAttribute{
				MarkdownDescription: "Network adapter list of associated IPv4 addresses",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			KeyAssign: schema.BoolAttribute{
				MarkdownDescription: "Whether an IP address should be automatically assigned to the adapter. Useless if addresses have been specified",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},

			KeyReserved: schema.BoolAttribute{
				MarkdownDescription: "Whether the network adapter is reserved (e.g. router), i.e. where the same hardware address can be reused over several subnets",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts adapter from Terraform model to Kowabunga API model
func adapterResourceToModel(d *AdapterResourceModel) models.Adapter {
	addresses := []string{}
	d.Addresses.ElementsAs(context.TODO(), &addresses, false)
	return models.Adapter{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Mac:         d.MAC.ValueString(),
		Addresses:   addresses,
		Reserved:    d.Reserved.ValueBoolPointer(),
	}
}

// converts adapter from Kowabunga API model to Terraform model
func adapterModelToResource(r *models.Adapter, d *AdapterResourceModel) {
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.MAC = types.StringValue(r.Mac)
	addresses := []attr.Value{}
	for _, a := range r.Addresses {
		addresses = append(addresses, types.StringValue(a))
	}
	d.Addresses, _ = types.ListValue(types.StringType, addresses)
	d.Reserved = types.BoolPointerValue(r.Reserved)
}

func (r *AdapterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *AdapterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	// find parent subnet
	subnetId, err := getSubnetID(r.Data, data.Subnet.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	// create a new adapter
	cfg := adapterResourceToModel(data)
	params := subnet.NewCreateAdapterParams().WithSubnetID(subnetId).WithBody(&cfg)
	if data.Assign.ValueBool() && len(cfg.Addresses) == 0 {
		params = params.WithAssignIP(data.Assign.ValueBoolPointer())
	}

	obj, err := r.Data.K.Subnet.CreateAdapter(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	adapterModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created adapter resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AdapterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *AdapterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := adapter.NewGetAdapterParams().WithAdapterID(data.ID.ValueString())
	obj, err := r.Data.K.Adapter.GetAdapter(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	adapterModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AdapterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *AdapterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := adapterResourceToModel(data)
	params := adapter.NewUpdateAdapterParams().WithAdapterID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Adapter.UpdateAdapter(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AdapterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *AdapterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := adapter.NewDeleteAdapterParams().WithAdapterID(data.ID.ValueString())
	_, err := r.Data.K.Adapter.DeleteAdapter(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
