package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/client/subnet"
	"github.com/dalet-oss/kowabunga-api/client/vnet"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	SubnetResourceName = "subnet"
)

var _ resource.Resource = &SubnetResource{}
var _ resource.ResourceWithImportState = &SubnetResource{}

func NewSubnetResource() resource.Resource {
	return &SubnetResource{}
}

type SubnetResource struct {
	Data *KowabungaProviderData
}

type SubnetResourceModel struct {
	ID      types.String             `tfsdk:"id"`
	Name    types.String             `tfsdk:"name"`
	Desc    types.String             `tfsdk:"desc"`
	VNet    types.String             `tfsdk:"vnet"`
	CIDR    types.String             `tfsdk:"cidr"`
	Gateway types.String             `tfsdk:"gateway"`
	DNS     types.String             `tfsdk:"dns"`
	DHCP    []DhcpRangeResourceModel `tfsdk:"dhcp"`
	Default types.Bool               `tfsdk:"default"`
}

type DhcpRangeResourceModel struct {
	First types.String `tfsdk:"first"`
	Last  types.String `tfsdk:"last"`
}

func (r *SubnetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, SubnetResourceName)
}

func (r *SubnetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *SubnetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *SubnetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a subnet resource",
		Attributes: map[string]schema.Attribute{
			KeyVNet: schema.StringAttribute{
				MarkdownDescription: "Associated virtual network name or ID",
				Required:            true,
			},
			KeyCIDR: schema.StringAttribute{
				MarkdownDescription: "Subnet CIDR",
				Required:            true,
			},
			KeyGateway: schema.StringAttribute{
				MarkdownDescription: "Subnet router/gateway",
				Required:            true,
			},
			KeyDNS: schema.StringAttribute{
				MarkdownDescription: "Subnet DNS server",
				Required:            true,
			},
			KeyDHCP: schema.ListNestedAttribute{
				MarkdownDescription: "List of subnet's dynamic DHCP ranges",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						KeyFirst: schema.StringAttribute{
							MarkdownDescription: "The range's first IP address for DHCP dynamic leases",
							Required:            true,
						},
						KeyLast: schema.StringAttribute{
							MarkdownDescription: "The range's last IP address for DHCP dynamic leases",
							Required:            true,
						},
					},
				},
			},
			KeyDefault: schema.BoolAttribute{
				MarkdownDescription: "Whether to set subnet as virtual network's default one",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts subnet from Terraform model to Kowabunga API model
func subnetResourceToModel(d *SubnetResourceModel) models.Subnet {
	dhcpRanges := []*models.DhcpRange{}
	for _, item := range d.DHCP {
		dr := models.DhcpRange{
			First: item.First.ValueStringPointer(),
			Last:  item.Last.ValueStringPointer(),
		}
		dhcpRanges = append(dhcpRanges, &dr)
	}

	return models.Subnet{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Cidr:        d.CIDR.ValueStringPointer(),
		Gateway:     d.Gateway.ValueStringPointer(),
		DNS:         d.DNS.ValueString(),
		Dhcp:        dhcpRanges,
	}
}

// converts subnet from Kowabunga API model to Terraform model
func subnetModelToResource(r *models.Subnet, d *SubnetResourceModel) {
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.CIDR = types.StringPointerValue(r.Cidr)
	d.Gateway = types.StringPointerValue(r.Gateway)
	d.DNS = types.StringValue(r.DNS)

	for _, item := range r.Dhcp {
		dr := DhcpRangeResourceModel{
			First: types.StringPointerValue(item.First),
			Last:  types.StringPointerValue(item.Last),
		}
		d.DHCP = append(d.DHCP, dr)
	}
}

func (r *SubnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *SubnetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	// find parent vnet
	vnetId, err := getVNetID(r.Data, data.VNet.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	// create a new subnet
	cfg := subnetResourceToModel(data)
	params := vnet.NewCreateSubnetParams().WithVnetID(vnetId).WithBody(&cfg)
	obj, err := r.Data.K.Vnet.CreateSubnet(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	// set virtual network as default
	if data.Default.ValueBool() {
		params2 := vnet.NewUpdateVNetDefaultSubnetParams().WithVnetID(vnetId).WithSubnetID(obj.Payload.ID)
		_, err = r.Data.K.Vnet.UpdateVNetDefaultSubnet(params2, nil)
		if err != nil {
			errorCreateGeneric(resp, err)
			return
		}
	}

	data.ID = types.StringValue(obj.Payload.ID)
	tflog.Trace(ctx, "created subnet resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *SubnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := subnet.NewGetSubnetParams().WithSubnetID(data.ID.ValueString())
	obj, err := r.Data.K.Subnet.GetSubnet(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	subnetModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubnetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *SubnetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := subnetResourceToModel(data)
	params := subnet.NewUpdateSubnetParams().WithSubnetID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Subnet.UpdateSubnet(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubnetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *SubnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := subnet.NewDeleteSubnetParams().WithSubnetID(data.ID.ValueString())
	_, err := r.Data.K.Subnet.DeleteSubnet(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
