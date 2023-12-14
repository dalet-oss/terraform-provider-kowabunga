package provider

import (
	"context"
	"fmt"

	"github.com/3th1nk/cidr"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/adapter"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/subnet"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
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
	ID             types.String   `tfsdk:"id"`
	Timeouts       timeouts.Value `tfsdk:"timeouts"`
	Name           types.String   `tfsdk:"name"`
	Desc           types.String   `tfsdk:"desc"`
	Subnet         types.String   `tfsdk:"subnet"`
	MAC            types.String   `tfsdk:"hwaddress"`
	Addresses      types.List     `tfsdk:"addresses"`
	Assign         types.Bool     `tfsdk:"assign"`
	Reserved       types.Bool     `tfsdk:"reserved"`
	CIDR           types.String   `tfsdk:"cidr"`
	Netmask        types.String   `tfsdk:"netmask"`
	NetmaskBitSize types.Int64    `tfsdk:"netmask_bitsize"`
	Gateway        types.String   `tfsdk:"gateway"`
}

func (r *AdapterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, AdapterResourceName)
}

func (r *AdapterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyCIDR), req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyNetmask), req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyNetmaskBitSize), req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyGateway), req, resp)
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
				MarkdownDescription: "Whether an IP address should be automatically assigned to the adapter (default: **true). Useless if addresses have been specified",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
			},

			KeyReserved: schema.BoolAttribute{
				MarkdownDescription: "Whether the network adapter is reserved (e.g. router), i.e. where the same hardware address can be reused over several subnets (default: **false**)",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
			KeyCIDR: schema.StringAttribute{
				MarkdownDescription: "Network mask CIDR (read-only), e.g. 192.168.0.0/24",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyNetmask: schema.StringAttribute{
				MarkdownDescription: "Network mask (read-only), e.g. 255.255.255.0",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyNetmaskBitSize: schema.Int64Attribute{
				MarkdownDescription: "Network mask size (read-only), e.g 24",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			KeyGateway: schema.StringAttribute{
				MarkdownDescription: "Network Gateway (read-only)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
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
	if r == nil {
		return
	}

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

func ipv4MaskString(m []byte) string {
	if len(m) != 4 {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
}

func (r *AdapterResource) GetSubnetData(data *AdapterResourceModel) error {
	// find real subnet id if a string was provided
	subnetId, err := getSubnetID(r.Data, data.Subnet.ValueString())
	if err != nil {
		return err
	}

	params := subnet.NewGetSubnetParams().WithSubnetID(subnetId)
	obj, err := r.Data.K.Subnet.GetSubnet(params, nil)
	if err != nil {
		return err
	}

	data.CIDR = types.StringPointerValue(obj.Payload.Cidr)

	c, err := cidr.Parse(*obj.Payload.Cidr)
	if err != nil {
		return err
	}
	data.Netmask = types.StringValue(ipv4MaskString(c.Mask()))
	size, _ := c.MaskSize()
	data.NetmaskBitSize = types.Int64Value(int64(size))
	data.Gateway = types.StringPointerValue(obj.Payload.Gateway)

	return nil
}

func (r *AdapterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *AdapterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := data.Timeouts.Create(ctx, DefaultCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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
	params := subnet.NewCreateAdapterParams().WithSubnetID(subnetId).WithBody(&cfg).WithTimeout(timeout)
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
	err = r.GetSubnetData(data)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	tflog.Trace(ctx, "created adapter resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AdapterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *AdapterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := data.Timeouts.Read(ctx, DefaultReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := adapter.NewGetAdapterParams().WithAdapterID(data.ID.ValueString()).WithTimeout(timeout)
	obj, err := r.Data.K.Adapter.GetAdapter(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}
	adapterModelToResource(obj.Payload, data)

	err = r.GetSubnetData(data)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AdapterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *AdapterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := data.Timeouts.Update(ctx, DefaultUpdateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := adapterResourceToModel(data)
	params := adapter.NewUpdateAdapterParams().WithAdapterID(data.ID.ValueString()).WithBody(&cfg).WithTimeout(timeout)
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
	timeout, diags := data.Timeouts.Delete(ctx, DefaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := adapter.NewDeleteAdapterParams().WithAdapterID(data.ID.ValueString()).WithTimeout(timeout)
	_, err := r.Data.K.Adapter.DeleteAdapter(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+params.AdapterID)
}
