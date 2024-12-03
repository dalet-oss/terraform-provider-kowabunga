package provider

import (
	"context"
	"maps"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	KawaiiIPSecResourceName = "kawaii_ipsec"

	KawaiiIPSecDefaultValueProtocol      = "tcp"
	KawaiiIPSecDefaultValueIngressPolicy = "accept"
)

var _ resource.Resource = &KawaiiResource{}
var _ resource.ResourceWithImportState = &KawaiiIPSecConnectionResource{}

func NewKawaiiIPSecResource() resource.Resource {
	return &KawaiiIPSecConnectionResource{}
}

type KawaiiIPSecConnectionResource struct {
	Data *KowabungaProviderData
}

type KawaiiIPSecConnectionResourceModel struct {
	ID       types.String   `tfsdk:"id"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
	Desc     types.String   `tfsdk:"desc"`

	KawaiiID     types.String `tfsdk:"kawaii_id"`
	Name         types.String `tfsdk:"name"`
	PreSharedKey types.String `tfsdk:"pre_shared_key"`
	RemotePeer   types.String `tfsdk:"remote_peer"`
	RemoteSubnet types.String `tfsdk:"remote_subnet"`
	IngressRules types.List   `tfsdk:"ingress_rules"` // KawaiiForwardRule
}

func (r *KawaiiIPSecConnectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, KawaiiIPSecResourceName)
}

func (r *KawaiiIPSecConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *KawaiiIPSecConnectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *KawaiiIPSecConnectionResource) SchemaIngressRule() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			KeySource: schema.StringAttribute{
				MarkdownDescription: "The source IP or CIDR to accept public traffic from (defaults to 0.0.0.0/0).",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(KawaiiDefaultValueSource),
				Validators: []validator.String{
					&stringNetworkAddressValidator{},
				},
			},
			KeyProtocol: schema.StringAttribute{
				MarkdownDescription: "The transport layer protocol to forward public traffic to (defaults to 'tcp')",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(KawaiiIPSecDefaultValueProtocol),
				Validators: []validator.String{
					&stringNetworkProtocolValidator{},
				},
			},
			KeyPorts: schema.StringAttribute{
				MarkdownDescription: "The ports (or range of ports) allowed to receive traffic. Ranges are accepted. Format is a-b,c-d (e.g. 443; 22,80,443; 80,443,3000-3005).",
				Required:            true,
				Validators: []validator.String{
					&stringNetworkPortRangesValidator{},
				},
			},
		},
	}
}

func (r *KawaiiIPSecConnectionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Kawaii list of Kowabunga IPSec Connections",
		Attributes: map[string]schema.Attribute{
			KeyKawaiiID: schema.StringAttribute{
				MarkdownDescription: "Associated Kawaii ID",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			KeyName: schema.StringAttribute{
				MarkdownDescription: "Kowabunga IPSec Connection Name",
				Required:            true,
			},
			KeyRemotePeer: schema.StringAttribute{
				MarkdownDescription: "Remote VPN Gateway",
				Required:            true,
				Validators: []validator.String{
					&stringNetworkAddressValidator{},
				},
			},
			KeyPreSharedKey: schema.StringAttribute{
				MarkdownDescription: "The Pre-Shared Key (PSK) to authenticate the VPN tunnel to your peer VPN gateway",
				Required:            true,
			},
			KeyRemoteSubnet: schema.StringAttribute{
				MarkdownDescription: "Remote Subnet",
				Required:            true,
				Validators: []validator.String{
					&stringNetworkAddressValidator{},
				},
			},
			KeyIngressRules: schema.ListNestedAttribute{
				MarkdownDescription: "The firewall list of Ingress Rules. Default will accept all. Egress is allow all",
				Optional:            true,
				Computed:            true,
				NestedObject:        r.SchemaIngressRule(),
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// ////////////////////////////////////////////////////////////////////
// converts kawai Ipsec from Terraform model to Kowabunga API model //
// ////////////////////////////////////////////////////////////////////
func kawaiiIPSecResourceModel(ctx *context.Context, d *KawaiiIPSecConnectionResourceModel) sdk.KawaiiIpSec {

	return sdk.KawaiiIpSec{
		Name:         d.Name.ValueString(),
		Description:  d.Desc.ValueStringPointer(),
		RemoteIp:     d.RemotePeer.ValueString(),
		RemoteSubnet: d.RemoteSubnet.ValueString(),
		PreSharedKey: d.PreSharedKey.ValueString(),
		Firewall:     kawaiiIPSecFirewallModel(ctx, d),
	}
}

func kawaiiIPSecFirewallModel(ctx *context.Context, d *KawaiiIPSecConnectionResourceModel) sdk.KawaiiFirewall {
	fwModel := sdk.KawaiiFirewall{
		Ingress: []sdk.KawaiiFirewallIngressRule{},
	}

	// Ingress Rules
	ingressRules := make([]types.Object, 0, len(d.IngressRules.Elements()))
	ingressDiags := d.IngressRules.ElementsAs(*ctx, &ingressRules, false)
	if ingressDiags.HasError() {
		for _, err := range ingressDiags.Errors() {
			tflog.Debug(*ctx, err.Detail())
		}
	}
	for _, ir := range ingressRules {
		rule := KawaiiIngressRule{}
		diags := ir.As(*ctx, &rule, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		})
		if diags.HasError() {
			for _, err := range diags.Errors() {
				tflog.Error(*ctx, err.Detail())
			}
		}

		fwModel.Ingress = append(fwModel.Ingress, sdk.KawaiiFirewallIngressRule{
			Source:   rule.Source.ValueStringPointer(),
			Protocol: rule.Protocol.ValueStringPointer(),
			Ports:    rule.Ports.ValueString(),
		})
	}
	return fwModel
}

/////////////////////////////////////////////////////////////////
// converts kawaii from Kowabunga API model to Terraform model //
/////////////////////////////////////////////////////////////////

func kawaiiIPSecModelToIngressRules(ctx *context.Context, r *sdk.KawaiiIpSec, d *KawaiiIPSecConnectionResourceModel) {
	// ingress rules
	ingressRules := []attr.Value{}
	ingressRuleType := map[string]attr.Type{
		KeySource:   types.StringType,
		KeyProtocol: types.StringType,
		KeyPorts:    types.StringType,
	}
	for _, ir := range r.Firewall.Ingress {
		source := KawaiiDefaultValueSource
		if ir.Source != nil {
			source = *ir.Source
		}
		protocol := KawaiiDefaultValueProtocol
		if ir.Protocol != nil {
			protocol = *ir.Protocol
		}
		r := map[string]attr.Value{
			KeySource:   types.StringValue(source),
			KeyProtocol: types.StringValue(protocol),
			KeyPorts:    types.StringValue(ir.Ports),
		}
		object, _ := types.ObjectValue(ingressRuleType, r)
		ingressRules = append(ingressRules, object)
	}

	if len(r.Firewall.Ingress) == 0 {
		d.IngressRules = types.ListNull(types.ObjectType{AttrTypes: ingressRuleType})
	} else {

		d.IngressRules, _ = types.ListValue(types.ObjectType{AttrTypes: ingressRuleType}, ingressRules)
	}
}

func kawaiiIPSecModelToResource(ctx *context.Context, r *sdk.KawaiiIpSec, d *KawaiiIPSecConnectionResourceModel) {
	if r == nil {
		return
	}
	d.Name = types.StringValue(r.Name)
	d.RemotePeer = types.StringValue(r.RemoteIp)
	d.RemoteSubnet = types.StringValue(r.RemoteSubnet)
	d.PreSharedKey = types.StringValue(r.PreSharedKey)
	if r.Description != nil {
		d.Desc = types.StringPointerValue(r.Description)
	} else {
		d.Desc = types.StringValue("")
	}
	kawaiiIPSecModelToIngressRules(ctx, r, d)
}

//////////////////////////////
// Terraform CRUD Functions //
//////////////////////////////

func (r *KawaiiIPSecConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *KawaiiIPSecConnectionResourceModel

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
	kawaiiId, err := getKawaiiID(ctx, r.Data, data.KawaiiID.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// create a new Kawaii IpSec Connection
	m := kawaiiIPSecResourceModel(&ctx, data)
	kawaiiIpSec, _, err := r.Data.K.KawaiiAPI.CreateKawaiiIpSec(ctx, kawaiiId).KawaiiIpSec(m).Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringPointerValue(kawaiiIpSec.Id)
	kawaiiIPSecModelToResource(&ctx, kawaiiIpSec, data) // read back resulting object
	tflog.Trace(ctx, "created Kawaii IPSec Tunnel resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KawaiiIPSecConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *KawaiiIPSecConnectionResourceModel
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

	kawaiiIpSec, _, err := r.Data.K.KawaiiAPI.ReadKawaiiIpSec(ctx, data.KawaiiID.ValueString(), data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	kawaiiIPSecModelToResource(&ctx, kawaiiIpSec, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KawaiiIPSecConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *KawaiiIPSecConnectionResourceModel
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

	m := kawaiiIPSecResourceModel(&ctx, data)
	_, _, err := r.Data.K.KawaiiAPI.UpdateKawaiiIpSec(ctx, data.KawaiiID.ValueString(), data.ID.ValueString()).KawaiiIpSec(m).Execute()
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KawaiiIPSecConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *KawaiiIPSecConnectionResourceModel
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

	_, err := r.Data.K.KawaiiAPI.DeleteKawaiiIpSec(ctx, data.KawaiiID.ValueString(), data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
