package provider

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/subnet"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/vnet"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	ID       types.String   `tfsdk:"id"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
	Name     types.String   `tfsdk:"name"`
	Desc     types.String   `tfsdk:"desc"`
	VNet     types.String   `tfsdk:"vnet"`
	CIDR     types.String   `tfsdk:"cidr"`
	Gateway  types.String   `tfsdk:"gateway"`
	DNS      types.String   `tfsdk:"dns"`
	Reserved types.List     `tfsdk:"reserved"`
	Routes   types.List     `tfsdk:"routes"`
	Default  types.Bool     `tfsdk:"default"`
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
			KeyReserved: schema.ListAttribute{
				MarkdownDescription: "List of subnet's reserved IPv4 ranges (format: 192.168.0.200-192.168.0.240)",
				Required:            true,
				ElementType:         types.StringType,
			},
			KeyRoutes: schema.ListAttribute{
				MarkdownDescription: "List of extra routes to be access through designated gateway (format: 10.0.0.0/8).",
				Required:            true,
				ElementType:         types.StringType,
			},
			KeyDefault: schema.BoolAttribute{
				MarkdownDescription: "Whether to set subnet as virtual network's default one (default: **false**). The first subnet to be created is always considered as default one.",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts subnet from Terraform model to Kowabunga API model
func subnetResourceToModel(d *SubnetResourceModel) models.Subnet {
	reservedRanges := []*models.IPRange{}
	ranges := []string{}
	d.Reserved.ElementsAs(context.TODO(), &ranges, false)
	for _, item := range ranges {
		split := strings.Split(item, "-")
		if len(split) != 2 {
			continue
		}
		ipr := models.IPRange{
			First: &split[0],
			Last:  &split[1],
		}
		reservedRanges = append(reservedRanges, &ipr)
	}
	routes := []string{}
	d.Routes.ElementsAs(context.TODO(), &routes, false)

	return models.Subnet{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Cidr:        d.CIDR.ValueStringPointer(),
		Gateway:     d.Gateway.ValueStringPointer(),
		DNS:         d.DNS.ValueString(),
		Reserved:    reservedRanges,
		ExtraRoutes: routes,
	}
}

// converts subnet from Kowabunga API model to Terraform model
func subnetModelToResource(s *models.Subnet, d *SubnetResourceModel) {
	if s == nil {
		return
	}

	d.Name = types.StringPointerValue(s.Name)
	d.Desc = types.StringValue(s.Description)
	d.CIDR = types.StringPointerValue(s.Cidr)
	d.Gateway = types.StringPointerValue(s.Gateway)
	d.DNS = types.StringValue(s.DNS)

	ranges := []attr.Value{}
	for _, item := range s.Reserved {
		ipr := fmt.Sprintf("%s-%s", *item.First, *item.Last)
		ranges = append(ranges, types.StringValue(ipr))
	}
	d.Reserved, _ = types.ListValue(types.StringType, ranges)
	routes := []attr.Value{}
	for _, r := range s.ExtraRoutes {
		routes = append(routes, types.StringValue(r))
	}
	d.Routes, _ = types.ListValue(types.StringType, routes)
}

func (r *SubnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *SubnetResourceModel
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

	// find parent vnet
	vnetId, err := getVNetID(r.Data, data.VNet.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// create a new subnet
	cfg := subnetResourceToModel(data)
	params := vnet.NewCreateSubnetParams().WithVnetID(vnetId).WithBody(&cfg).WithTimeout(timeout)
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
	//subnetModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created subnet resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *SubnetResourceModel
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

	params := subnet.NewGetSubnetParams().WithSubnetID(data.ID.ValueString()).WithTimeout(timeout)
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

	timeout, diags := data.Timeouts.Update(ctx, DefaultUpdateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := subnetResourceToModel(data)
	params := subnet.NewUpdateSubnetParams().WithSubnetID(data.ID.ValueString()).WithBody(&cfg).WithTimeout(timeout)
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

	timeout, diags := data.Timeouts.Delete(ctx, DefaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := subnet.NewDeleteSubnetParams().WithSubnetID(data.ID.ValueString()).WithTimeout(timeout)
	_, err := r.Data.K.Subnet.DeleteSubnet(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted")
}
