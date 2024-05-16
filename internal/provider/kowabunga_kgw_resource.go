package provider

import (
	"context"
	"sort"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go/client"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/exp/maps"
)

const (
	KgwResourceName = "kgw"
)

var _ resource.Resource = &KgwResource{}
var _ resource.ResourceWithImportState = &KgwResource{}

func NewKgwResource() resource.Resource {
	return &KgwResource{}
}

type KgwResource struct {
	Data *KowabungaProviderData
}

type KgwResourceModel struct {
	ID       types.String   `tfsdk:"id"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
	Desc     types.String   `tfsdk:"desc"`
	Project  types.String   `tfsdk:"project"`

	Region       types.String `tfsdk:"region"`
	ZoneSettings types.List   `tfsdk:"zone_settings"`
	Nats         types.List   `tfsdk:"nats"`
	VnetPeerings types.List   `tfsdk:"vnet_peerings"`
}

type KgwZoneSettings struct {
	Zone      types.String `tfsdk:"zone"`
	PublicIp  types.String `tfsdk:"public_ip"`
	PrivateIp types.String `tfsdk:"private_ip"`
}

type KgwNatRule struct {
	PublicIp  types.String `tfsdk:"public_ip"`
	PrivateIp types.String `tfsdk:"private_ip"`
	Ports     types.String `tfsdk:"ports"`
}

type KgwVnetPeering struct {
	VNet   types.String `tfsdk:"vnet"`
	Subnet types.String `tfsdk:"subnet"`
	Ports  types.String `tfsdk:"ports"`
	IPs    types.List   `tfsdk:"ips"`
}

func (r *KgwResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, KgwResourceName)
}

func (r *KgwResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyZoneSettings), req, resp)
}

func (r *KgwResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *KgwResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a KGW resource. **KGW** (stands for *Kowabunga Gateway*) is a resource that provides Nats & internet access capabilities for a given project.",
		Attributes: map[string]schema.Attribute{
			KeyProject: schema.StringAttribute{
				MarkdownDescription: "Associated project name or ID",
				Required:            true,
			},
			KeyRegion: schema.StringAttribute{
				MarkdownDescription: "Associated region name or ID",
				Required:            true,
			},
			KeyZoneSettings: schema.ListNestedAttribute{
				MarkdownDescription: "Per-zone network settings (read-only)",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						KeyZone: schema.StringAttribute{
							MarkdownDescription: "Zone name (read-only)",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						KeyPublicIp: schema.StringAttribute{
							MarkdownDescription: "KGW zone-local public IP address (read-only)",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						KeyPrivateIp: schema.StringAttribute{
							MarkdownDescription: "KGW zone-local private IP address (read-only)",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			KeyNats: schema.ListNestedAttribute{
				MarkdownDescription: "NATs Configuration",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						KeyPrivateIp: schema.StringAttribute{
							MarkdownDescription: "Private IP where the NAT will be forwarded",
							Required:            true,
						},
						KeyPublicIp: schema.StringAttribute{
							MarkdownDescription: "Exposed public IP used to forward traffic. Leave empty to use the default GW interface",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						KeyPorts: schema.StringAttribute{
							MarkdownDescription: "Ports that will be forwarded. 0 Means all. For a list of ports, separate it with a comma, Ranges Accepted. e.g 8001,9006-9010",
							Required:            true,
							Validators: []validator.String{
								&stringPortValidator{},
							},
						},
					},
				},
			},
			KeyVnetPeerings: schema.ListNestedAttribute{
				MarkdownDescription: "Virtual Network Peerings Configuration",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						KeyVNet: schema.StringAttribute{
							MarkdownDescription: "Kowabunga VNet ID to be peered with",
							Required:            true,
						},
						KeySubnet: schema.StringAttribute{
							MarkdownDescription: "Kowabunga Subnet ID to be peered with (IP address will be automatically assigned)",
							Required:            true,
						},
						KeyPorts: schema.StringAttribute{
							MarkdownDescription: "Ports to be reachable from peered subnet. If specified, traffic will be filtered. For a list of ports, separate it with a comma, Ranges Accepted. e.g 8001,9006-9010",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
							Validators: []validator.String{
								&stringPortValidator{},
							},
						},
						KeyIPs: schema.ListAttribute{
							MarkdownDescription: "List of auto-assigned private IP addresses in peered subnet (read-only)",
							ElementType:         types.StringType,
							Required:            true,
						},
					},
				},
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributesWithoutName(&ctx))
}

// converts kgw from Terraform model to Kowabunga API model
func kgwZoneSettingsModel(ctx *context.Context, d *KgwResourceModel) []sdk.KGWZoneSettings {
	zsModel := []sdk.KGWZoneSettings{}

	zs := make([]types.Object, 0, len(d.ZoneSettings.Elements()))
	diags := d.ZoneSettings.ElementsAs(*ctx, &zs, false)
	if diags.HasError() {
		for _, err := range diags.Errors() {
			tflog.Debug(*ctx, err.Detail())
		}
	}

	zoneSettings := KgwZoneSettings{}
	for _, z := range zs {
		diags := z.As(*ctx, &zoneSettings, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		})
		if diags.HasError() {
			for _, err := range diags.Errors() {
				tflog.Error(*ctx, err.Detail())
			}
		}
		zsModel = append(zsModel, sdk.KGWZoneSettings{
			Zone:      zoneSettings.Zone.ValueString(),
			PublicIp:  zoneSettings.PublicIp.ValueString(),
			PrivateIp: zoneSettings.PrivateIp.ValueString(),
		})
	}

	return zsModel
}

func kgwNatsModel(ctx *context.Context, d *KgwResourceModel) []sdk.KGWNat {
	natsModel := []sdk.KGWNat{}

	nats := make([]types.Object, 0, len(d.Nats.Elements()))
	diags := d.Nats.ElementsAs(*ctx, &nats, false)
	if diags.HasError() {
		for _, err := range diags.Errors() {
			tflog.Debug(*ctx, err.Detail())
		}
	}

	natRule := KgwNatRule{}
	for _, nat := range nats {
		diags := nat.As(*ctx, &natRule, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		})
		if diags.HasError() {
			for _, err := range diags.Errors() {
				tflog.Error(*ctx, err.Detail())
			}
		}
		natsModel = append(natsModel, sdk.KGWNat{
			PrivateIp: natRule.PrivateIp.ValueString(),
			PublicIp:  natRule.PublicIp.ValueStringPointer(),
			Ports:     natRule.Ports.ValueStringPointer(),
		})
	}

	return natsModel
}

func kgwVnetPeeringsModel(ctx *context.Context, d *KgwResourceModel) []sdk.KGWVnetPeering {
	vpModel := []sdk.KGWVnetPeering{}

	peerings := make([]types.Object, 0, len(d.VnetPeerings.Elements()))
	diags := d.VnetPeerings.ElementsAs(*ctx, &peerings, false)
	if diags.HasError() {
		for _, err := range diags.Errors() {
			tflog.Debug(*ctx, err.Detail())
		}
	}

	vp := KgwVnetPeering{}
	for _, p := range peerings {
		diags := p.As(*ctx, &vp, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		})
		if diags.HasError() {
			for _, err := range diags.Errors() {
				tflog.Error(*ctx, err.Detail())
			}
		}

		ips := []string{}
		vp.IPs.ElementsAs(context.TODO(), &ips, false)
		sort.Strings(ips)

		vpModel = append(vpModel, sdk.KGWVnetPeering{
			Vnet:   vp.VNet.ValueString(),
			Subnet: vp.Subnet.ValueString(),
			Ports:  vp.Ports.ValueStringPointer(),
			Ips:    ips,
		})
	}

	return vpModel
}

func kgwResourceToModel(ctx *context.Context, d *KgwResourceModel) sdk.KGW {
	return sdk.KGW{
		Description:  d.Desc.ValueStringPointer(),
		Addresses:    kgwZoneSettingsModel(ctx, d),
		Nats:         kgwNatsModel(ctx, d),
		VnetPeerings: kgwVnetPeeringsModel(ctx, d),
	}
}

func kgwModelToZoneSettings(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	zs := []attr.Value{}
	zsType := map[string]attr.Type{
		KeyZone:      types.StringType,
		KeyPublicIp:  types.StringType,
		KeyPrivateIp: types.StringType,
	}
	for _, z := range r.Addresses {
		a := map[string]attr.Value{
			KeyZone:      types.StringValue(z.Zone),
			KeyPublicIp:  types.StringValue(z.PublicIp),
			KeyPrivateIp: types.StringValue(z.PrivateIp),
		}
		object, _ := types.ObjectValue(zsType, a)
		zs = append(zs, object)
	}

	if len(r.Addresses) == 0 {
		d.ZoneSettings = types.ListNull(types.ObjectType{AttrTypes: zsType})
	} else {
		d.ZoneSettings, _ = types.ListValue(types.ObjectType{AttrTypes: zsType}, zs)
	}
}

func kgwModelToNats(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	nats := []attr.Value{}
	natType := map[string]attr.Type{
		KeyPrivateIp: types.StringType,
		KeyPublicIp:  types.StringType,
		KeyPorts:     types.StringType,
	}
	for _, nat := range r.Nats {
		var publicIp string
		if nat.PublicIp != nil {
			publicIp = *nat.PublicIp
		}
		var ports string
		if nat.Ports != nil {
			ports = *nat.Ports
		}
		a := map[string]attr.Value{
			KeyPrivateIp: types.StringValue(nat.PrivateIp),
			KeyPublicIp:  types.StringPointerValue(&publicIp),
			KeyPorts:     types.StringPointerValue(&ports),
		}
		object, _ := types.ObjectValue(natType, a)
		nats = append(nats, object)
	}

	if len(r.Nats) == 0 {
		d.Nats = types.ListNull(types.ObjectType{AttrTypes: natType})
	} else {
		d.Nats, _ = types.ListValue(types.ObjectType{AttrTypes: natType}, nats)
	}
}

func kgwModelToVnetPeerings(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	vps := []attr.Value{}
	vpType := map[string]attr.Type{
		KeyVNet:   types.StringType,
		KeySubnet: types.StringType,
		KeyPorts:  types.StringType,
	}
	for _, v := range r.VnetPeerings {
		var ports string
		if v.Ports != nil {
			ports = *v.Ports
		}

		ips := []attr.Value{}
		sort.Strings(v.Ips)
		for _, i := range v.Ips {
			ips = append(ips, types.StringValue(i))
		}

		a := map[string]attr.Value{
			KeyVNet:   types.StringValue(v.Vnet),
			KeySubnet: types.StringValue(v.Subnet),
			KeyPorts:  types.StringPointerValue(&ports),
		}
		a[KeyIPs], _ = types.ListValue(types.StringType, ips)
		object, _ := types.ObjectValue(vpType, a)
		vps = append(vps, object)
	}

	if len(r.VnetPeerings) == 0 {
		d.VnetPeerings = types.ListNull(types.ObjectType{AttrTypes: vpType})
	} else {
		d.VnetPeerings, _ = types.ListValue(types.ObjectType{AttrTypes: vpType}, vps)
	}
}

// converts kgw from Kowabunga API model to Terraform model
func kgwModelToResource(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	if r == nil {
		return
	}
	if r.Description != nil {
		d.Desc = types.StringPointerValue(r.Description)
	} else {
		d.Desc = types.StringValue("")
	}

	kgwModelToZoneSettings(ctx, r, d)
	kgwModelToNats(ctx, r, d)
	kgwModelToVnetPeerings(ctx, r, d)
}

func (r *KgwResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *KgwResourceModel

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

	// find parent project
	projectId, err := getProjectID(ctx, r.Data, data.Project.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// find parent region
	regionId, err := getRegionID(ctx, r.Data, data.Region.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	m := kgwResourceToModel(&ctx, data)

	// create a new KGW
	kgw, _, err := r.Data.K.ProjectAPI.CreateProjectRegionKGW(ctx, projectId, regionId).KGW(m).Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringPointerValue(kgw.Id)
	kgwModelToResource(&ctx, kgw, data) // read back resulting object
	tflog.Trace(ctx, "created KGW resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KgwResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *KgwResourceModel
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

	kgw, _, err := r.Data.K.KgwAPI.ReadKGW(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	kgwModelToResource(&ctx, kgw, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KgwResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *KgwResourceModel
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

	m := kgwResourceToModel(&ctx, data)
	_, _, err := r.Data.K.KgwAPI.UpdateKGW(ctx, data.ID.ValueString()).KGW(m).Execute()
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KgwResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *KgwResourceModel
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

	_, err := r.Data.K.KgwAPI.DeleteKGW(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
