package provider

import (
	"context"
	"maps"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go/client"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	KgwResourceName = "kgw"

	KgwDefaultValueProtocol      = "tcp"
	KgwDefaultValueIngressPolicy = "drop"
	KgwDefaultValueEgressPolicy  = "accept"
	KgwDefaultValueForwardPolicy = "drop"
	KgwDefaultValueSource        = "0.0.0.0/0"
	KgwDefaultValueDestination   = "0.0.0.0/0"
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
	Region   types.String   `tfsdk:"region"`

	NetworkCfg   types.Object `tfsdk:"netcfg"`        // read-only
	IngressRules types.List   `tfsdk:"ingress_rules"` // KgwIngressRule
	EgressPolicy types.String `tfsdk:"egress_policy"`
	EgressRules  types.List   `tfsdk:"egress_rules"` // KgwEgressRule
	NatRules     types.List   `tfsdk:"nat_rules"`    // KgwNatRule
	VpcPeerings  types.List   `tfsdk:"vpc_peerings"` // KgwVpcPeering
}

type KgwNetworkConfig struct {
	PublicIPs  types.List `tfsdk:"public_ips"`  // []string
	PrivateIPs types.List `tfsdk:"private_ips"` // []string
	Zones      types.List `tfsdk:"zones"`       // KgwNetworkZoneConfig
}

type KgwNetworkZoneConfig struct {
	Zone      types.String `tfsdk:"zone"`
	PublicIp  types.String `tfsdk:"public_ip"`
	PrivateIp types.String `tfsdk:"private_ip"`
}

type KgwIngressRule struct {
	Source   types.String `tfsdk:"source"`
	Protocol types.String `tfsdk:"protocol"`
	Ports    types.String `tfsdk:"ports"`
}

type KgwEgressRule struct {
	Destination types.String `tfsdk:"destination"`
	Protocol    types.String `tfsdk:"protocol"`
	Ports       types.String `tfsdk:"ports"`
}

type KgwForwardRule struct {
	Protocol types.String `tfsdk:"protocol"`
	Ports    types.String `tfsdk:"ports"`
}

type KgwNatRule struct {
	Destination types.String `tfsdk:"destination"`
	Protocol    types.String `tfsdk:"protocol"`
	Ports       types.String `tfsdk:"ports"`
}

type KgwVpcPeering struct {
	Subnet       types.String `tfsdk:"subnet"`
	Policy       types.String `tfsdk:"policy"`
	IngressRules types.List   `tfsdk:"ingress_rules"` // KgwForwardRule
	EgressRules  types.List   `tfsdk:"egress_rules"`  // KgwForwardRule
	NetworkCfg   types.List   `tfsdk:"netcfg"`        // KgwVpcPeeringNetworkZoneConfig, read-only
}

type KgwVpcPeeringNetworkZoneConfig struct {
	Zone      types.String `tfsdk:"zone"`
	PrivateIp types.String `tfsdk:"private_ip"`
}

func (r *KgwResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, KgwResourceName)
}

func (r *KgwResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *KgwResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *KgwResource) SchemaNetworkZoneConfig() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: "KGW per-zone list of Kowabunga virtual IP addresses (read-only)",
		Required:            false,
		Optional:            false,
		Computed:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				KeyZone: schema.StringAttribute{
					MarkdownDescription: "KGW zone name (read-only)",
					Required:            false,
					Optional:            false,
					Computed:            true,
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.UseStateForUnknown(),
					},
				},
				KeyPublicIP: schema.StringAttribute{
					MarkdownDescription: "KGW zone gateway public virtual IP (read-only)",
					Required:            false,
					Optional:            false,
					Computed:            true,
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.UseStateForUnknown(),
					},
				},
				KeyPrivateIP: schema.StringAttribute{
					MarkdownDescription: "KGW zone gateway private virtual IP (read-only).",
					Required:            false,
					Optional:            false,
					Computed:            true,
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.UseStateForUnknown(),
					},
				},
			},
		},
	}
}

func (r *KgwResource) SchemaNetworkConfig() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "KGW list of assigned virtual IPs per-zone addresses (read-only)",
		Required:            false,
		Optional:            false,
		Computed:            true,
		Attributes: map[string]schema.Attribute{
			KeyPublicIPs: schema.ListAttribute{
				MarkdownDescription: "KGW global public gateways virtual IP addresses (read-only).",
				Required:            false,
				Optional:            false,
				Computed:            true,
				ElementType:         types.StringType,
			},
			KeyPrivateIPs: schema.ListAttribute{
				MarkdownDescription: "KGW global private gateways virtual IP addresses (read-only).",
				Required:            false,
				Optional:            false,
				Computed:            true,
				ElementType:         types.StringType,
			},
			KeyZones: r.SchemaNetworkZoneConfig(),
		},
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
	}
}

func (r *KgwResource) SchemaIngressRules() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: "The KGW public firewall list of ingress rules. KGW default policy is to drop all incoming traffic, including ICMP. Specified ruleset will be explicitly accepted.",
		Optional:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				KeySource: schema.StringAttribute{
					MarkdownDescription: "The source IP or CIDR to accept public traffic from (defaults to 0.0.0.0/0).",
					Optional:            true,
					Computed:            true,
					Default:             stringdefault.StaticString(KgwDefaultValueSource),
					Validators: []validator.String{
						&stringNetworkAddressValidator{},
					},
				},
				KeyProtocol: schema.StringAttribute{
					MarkdownDescription: "The transport layer protocol to accept public traffic from (defaults to 'tcp').",
					Optional:            true,
					Computed:            true,
					Default:             stringdefault.StaticString(KgwDefaultValueProtocol),
					Validators: []validator.String{
						&stringNetworkProtocolValidator{},
					},
				},
				KeyPorts: schema.StringAttribute{
					MarkdownDescription: "The port (or list of ports) to accept public traffic from. Ranges are accepted. Format is a-b,c-d (e.g. 443; 22,80,443; 80,443,3000-3005).",
					Required:            true,
					Validators: []validator.String{
						&stringNetworkPortRangesValidator{},
					},
				},
			},
		},
	}
}

func (r *KgwResource) SchemaEgressRules() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: "KGW public firewall list of egress rules. KGW default policy is to accept all outgoing traffic, including ICMP. Specified ruleset will be explicitly dropped if egress_policy is set to accept, and explicitly accepted if egress policy is set to drop.",
		Optional:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				KeyDestination: schema.StringAttribute{
					MarkdownDescription: "The destination IP or CIDR to accept/drop public traffic to (defaults to 0.0.0.0/0) ",
					Optional:            true,
					Computed:            true,
					Default:             stringdefault.StaticString(KgwDefaultValueDestination),
					Validators: []validator.String{
						&stringNetworkAddressValidator{},
					},
				},
				KeyProtocol: schema.StringAttribute{
					MarkdownDescription: "The transport layer protocol to accept/drop public traffic to (defaults to 'tcp')",
					Optional:            true,
					Computed:            true,
					Default:             stringdefault.StaticString(KgwDefaultValueProtocol),
					Validators: []validator.String{
						&stringNetworkProtocolValidator{},
					},
				},
				KeyPorts: schema.StringAttribute{
					MarkdownDescription: "The port (or list of ports) to forward public traffic from. Ranges are accepted. Format is a-b,c-d (e.g. 443; 22,80,443; 80,443,3000-3005).",
					Required:            true,
					Validators: []validator.String{
						&stringNetworkPortRangesValidator{},
					},
				},
			},
		},
	}
}

func (r *KgwResource) SchemaForwardRule() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			KeyProtocol: schema.StringAttribute{
				MarkdownDescription: "The transport layer protocol to forward public traffic to (defaults to 'tcp')",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(KgwDefaultValueProtocol),
				Validators: []validator.String{
					&stringNetworkProtocolValidator{},
				},
			},
			KeyPorts: schema.StringAttribute{
				MarkdownDescription: "The port (or list of ports) to forward public traffic from. Ranges are accepted. Format is a-b,c-d (e.g. 443; 22,80,443; 80,443,3000-3005).",
				Required:            true,
				Validators: []validator.String{
					&stringNetworkPortRangesValidator{},
				},
			},
		},
	}
}

func (r *KgwResource) SchemaNatRules() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: "KGW list of NAT forwarding rules. KGW will forward public Internet traffic from all public virtual IPs to requested private subnet IP addresses.",
		Optional:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				KeyDestination: schema.StringAttribute{
					MarkdownDescription: "Target private IP address to forward public traffic to.",
					Required:            true,
				},
				KeyProtocol: schema.StringAttribute{
					MarkdownDescription: "The transport layer protocol to forward public traffic to (defaults to 'tcp')",
					Optional:            true,
					Computed:            true,
					Default:             stringdefault.StaticString(KgwDefaultValueProtocol),
					Validators: []validator.String{
						&stringNetworkProtocolValidator{},
					},
				},
				KeyPorts: schema.StringAttribute{
					MarkdownDescription: "The port (or list of ports) to forward public traffic from. Ranges are accepted. Format is a-b,c-d (e.g. 443; 22,80,443; 80,443,3000-3005).",
					Required:            true,
					Validators: []validator.String{
						&stringNetworkPortRangesValidator{},
					},
				},
			},
		},
	}
}

func (r *KgwResource) SchemaVpcPeerings() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: "KGW list of Kowabunga private VPC subnet peering rules.",
		Optional:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				KeySubnet: schema.StringAttribute{
					MarkdownDescription: "Kowabunga Subnet ID to be peered with (subnet local IP addresses will be automatically assigned to KGW instances).",
					Required:            true,
				},
				KeyPolicy: schema.StringAttribute{
					MarkdownDescription: "The default VPC traffic forwarding policy: 'accept' (default) or 'drop'",
					Optional:            true,
					Computed:            true,
					Default:             stringdefault.StaticString(KgwDefaultValueForwardPolicy),
					Validators: []validator.String{
						&stringFirewallPolicyValidator{},
					},
				},
				KeyIngressRules: schema.ListNestedAttribute{
					MarkdownDescription: "The firewall list of forwarding ingress rules from VPC peered subnet. ICMP traffic is always accepted. The specified ruleset will be explicitly accepted if drop is the default policy (useless otherwise)",
					Optional:            true,
					Computed:            true,
					NestedObject:        r.SchemaForwardRule(),
					PlanModifiers: []planmodifier.List{
						listplanmodifier.UseStateForUnknown(),
					},
				},
				KeyEgressRules: schema.ListNestedAttribute{
					MarkdownDescription: "The firewall list of forwarding egress rules to VPC peered subnet. ICMP trafficis always accepted. The specified ruleset will be explicitly accepted if drop is the default policy (useless otherwise)",
					Optional:            true,
					Computed:            true,
					NestedObject:        r.SchemaForwardRule(),
					PlanModifiers: []planmodifier.List{
						listplanmodifier.UseStateForUnknown(),
					},
				},
				KeyNetworkConfig: schema.ListNestedAttribute{
					MarkdownDescription: "The per-zone auto-assigned private IPs in peered subnet (read-only)",
					Required:            false,
					Optional:            false,
					Computed:            true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							KeyZone: schema.StringAttribute{
								MarkdownDescription: "KGW zone name (read-only).",
								Required:            false,
								Optional:            false,
								Computed:            true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
								},
							},
							KeyPrivateIP: schema.StringAttribute{
								MarkdownDescription: "KGW zone gateway private IP address in VPC peered subnet (read-only)",
								Required:            false,
								Optional:            false,
								Computed:            true,
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.UseStateForUnknown(),
								},
							},
						},
					},
					PlanModifiers: []planmodifier.List{
						listplanmodifier.UseStateForUnknown(),
					},
				},
			},
		},
		PlanModifiers: []planmodifier.List{
			listplanmodifier.UseStateForUnknown(),
		},
	}
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
			KeyNetworkConfig: r.SchemaNetworkConfig(),
			KeyIngressRules:  r.SchemaIngressRules(),
			KeyEgressPolicy: schema.StringAttribute{
				MarkdownDescription: "KGW default public traffic firewall egress policy: 'accept' (default) or 'drop'",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(KgwDefaultValueEgressPolicy),
				Validators: []validator.String{
					&stringFirewallPolicyValidator{},
				},
			},
			KeyEgressRules: r.SchemaEgressRules(),
			KeyNatRules:    r.SchemaNatRules(),
			KeyVpcPeerings: r.SchemaVpcPeerings(),
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributesWithoutName(&ctx))
}

//////////////////////////////////////////////////////////////
// converts kgw from Terraform model to Kowabunga API model //
//////////////////////////////////////////////////////////////

func kgwFirewallModel(ctx *context.Context, d *KgwResourceModel) sdk.KGWFirewall {
	fwModel := sdk.KGWFirewall{
		Ingress:      []sdk.KGWFirewallIngressRule{},
		EgressPolicy: d.EgressPolicy.ValueStringPointer(),
		Egress:       []sdk.KGWFirewallEgressRule{},
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
		rule := KgwIngressRule{}
		diags := ir.As(*ctx, &rule, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		})
		if diags.HasError() {
			for _, err := range diags.Errors() {
				tflog.Error(*ctx, err.Detail())
			}
		}

		fwModel.Ingress = append(fwModel.Ingress, sdk.KGWFirewallIngressRule{
			Source:   rule.Source.ValueStringPointer(),
			Protocol: rule.Protocol.ValueStringPointer(),
			Ports:    rule.Ports.ValueString(),
		})
	}

	// Egress Rules
	egressRules := make([]types.Object, 0, len(d.EgressRules.Elements()))
	egressDiags := d.EgressRules.ElementsAs(*ctx, &egressRules, false)
	if egressDiags.HasError() {
		for _, err := range egressDiags.Errors() {
			tflog.Debug(*ctx, err.Detail())
		}
	}
	for _, er := range egressRules {
		rule := KgwEgressRule{}
		diags := er.As(*ctx, &rule, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		})
		if diags.HasError() {
			for _, err := range diags.Errors() {
				tflog.Error(*ctx, err.Detail())
			}
		}

		fwModel.Egress = append(fwModel.Egress, sdk.KGWFirewallEgressRule{
			Destination: rule.Destination.ValueStringPointer(),
			Protocol:    rule.Protocol.ValueStringPointer(),
			Ports:       rule.Ports.ValueString(),
		})
	}

	return fwModel
}

func kgwNatRulesModel(ctx *context.Context, d *KgwResourceModel) []sdk.KGWDNatRule {
	natModel := []sdk.KGWDNatRule{}

	rules := make([]types.Object, 0, len(d.NatRules.Elements()))
	diags := d.NatRules.ElementsAs(*ctx, &rules, false)
	if diags.HasError() {
		for _, err := range diags.Errors() {
			tflog.Debug(*ctx, err.Detail())
		}
	}

	for _, r := range rules {
		rule := KgwNatRule{}
		diags := r.As(*ctx, &rule, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		})
		if diags.HasError() {
			for _, err := range diags.Errors() {
				tflog.Error(*ctx, err.Detail())
			}
		}
		natModel = append(natModel, sdk.KGWDNatRule{
			Destination: rule.Destination.ValueString(),
			Protocol:    rule.Protocol.ValueStringPointer(),
			Ports:       rule.Ports.ValueString(),
		})
	}

	return natModel
}

func kgwVpcPeeringsModel(ctx *context.Context, d *KgwResourceModel) []sdk.KGWVpcPeering {
	vpModel := []sdk.KGWVpcPeering{}

	peerings := make([]types.Object, 0, len(d.VpcPeerings.Elements()))
	diags := d.VpcPeerings.ElementsAs(*ctx, &peerings, false)
	if diags.HasError() {
		for _, err := range diags.Errors() {
			tflog.Debug(*ctx, err.Detail())
		}
	}

	for _, p := range peerings {
		vp := KgwVpcPeering{}
		diags := p.As(*ctx, &vp, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		})
		if diags.HasError() {
			for _, err := range diags.Errors() {
				tflog.Error(*ctx, err.Detail())
			}
		}

		// ingress rules
		ingressModel := []sdk.KGWVpcForwardRule{}
		ingressRules := make([]types.Object, 0, len(vp.IngressRules.Elements()))
		ingressDiags := vp.IngressRules.ElementsAs(*ctx, &ingressRules, false)
		if ingressDiags.HasError() {
			for _, err := range ingressDiags.Errors() {
				tflog.Debug(*ctx, err.Detail())
			}
		}

		for _, ir := range ingressRules {
			rule := KgwForwardRule{}
			diags := ir.As(*ctx, &rule, basetypes.ObjectAsOptions{
				UnhandledNullAsEmpty:    true,
				UnhandledUnknownAsEmpty: true,
			})
			if diags.HasError() {
				for _, err := range diags.Errors() {
					tflog.Error(*ctx, err.Detail())
				}
			}

			ingressModel = append(ingressModel, sdk.KGWVpcForwardRule{
				Protocol: rule.Protocol.ValueStringPointer(),
				Ports:    rule.Ports.ValueString(),
			})
		}

		// egress rules
		egressModel := []sdk.KGWVpcForwardRule{}
		egressRules := make([]types.Object, 0, len(vp.EgressRules.Elements()))
		egressDiags := vp.EgressRules.ElementsAs(*ctx, &egressRules, false)
		if egressDiags.HasError() {
			for _, err := range egressDiags.Errors() {
				tflog.Debug(*ctx, err.Detail())
			}
		}

		for _, er := range egressRules {
			rule := KgwForwardRule{}
			diags := er.As(*ctx, &rule, basetypes.ObjectAsOptions{
				UnhandledNullAsEmpty:    true,
				UnhandledUnknownAsEmpty: true,
			})
			if diags.HasError() {
				for _, err := range diags.Errors() {
					tflog.Error(*ctx, err.Detail())
				}
			}

			egressModel = append(egressModel, sdk.KGWVpcForwardRule{
				Protocol: rule.Protocol.ValueStringPointer(),
				Ports:    rule.Ports.ValueString(),
			})
		}

		vpModel = append(vpModel, sdk.KGWVpcPeering{
			Subnet:  vp.Subnet.ValueString(),
			Policy:  vp.Policy.ValueStringPointer(),
			Ingress: ingressModel,
			Egress:  egressModel,
		})
	}

	return vpModel
}

func kgwResourceToModel(ctx *context.Context, d *KgwResourceModel) sdk.KGW {
	return sdk.KGW{
		Description: d.Desc.ValueStringPointer(),
		Firewall:    kgwFirewallModel(ctx, d),
		Dnat:        kgwNatRulesModel(ctx, d),
		VpcPeerings: kgwVpcPeeringsModel(ctx, d),
	}
}

//////////////////////////////////////////////////////////////
// converts kgw from Kowabunga API model to Terraform model //
//////////////////////////////////////////////////////////////

func kgwModelToNetworkConfig(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	nc := map[string]attr.Value{}
	ncType := map[string]attr.Type{
		KeyPublicIPs: types.ListType{
			ElemType: types.StringType,
		},
		KeyPrivateIPs: types.ListType{
			ElemType: types.StringType,
		},
		KeyZones: types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					KeyZone:      types.StringType,
					KeyPublicIP:  types.StringType,
					KeyPrivateIP: types.StringType,
				},
			},
		},
	}

	// cross-zones global public IPs
	publicIPs := []attr.Value{}
	for _, pub := range r.Netip.Public {
		publicIPs = append(publicIPs, types.StringValue(pub))
	}
	nc[KeyPublicIPs], _ = types.ListValue(types.StringType, publicIPs)

	// cross-zones global private IPs
	privateIPs := []attr.Value{}
	for _, priv := range r.Netip.Private {
		privateIPs = append(privateIPs, types.StringValue(priv))
	}
	nc[KeyPrivateIPs], _ = types.ListValue(types.StringType, privateIPs)

	// zone-specific network configuration
	zoneNetCfg := []attr.Value{}
	zoneNetCfgType := map[string]attr.Type{
		KeyZone:      types.StringType,
		KeyPublicIP:  types.StringType,
		KeyPrivateIP: types.StringType,
	}
	for _, z := range r.Netip.Zones {
		v := map[string]attr.Value{
			KeyZone:      types.StringValue(z.Zone),
			KeyPublicIP:  types.StringValue(z.Public),
			KeyPrivateIP: types.StringValue(z.Private),
		}
		object, _ := types.ObjectValue(zoneNetCfgType, v)
		zoneNetCfg = append(zoneNetCfg, object)
	}
	if len(r.Netip.Zones) == 0 {
		nc[KeyZones] = types.ListNull(types.ObjectType{AttrTypes: zoneNetCfgType})
	} else {
		nc[KeyZones], _ = types.ListValue(types.ObjectType{AttrTypes: zoneNetCfgType}, zoneNetCfg)
	}

	// resulting object
	d.NetworkCfg, _ = types.ObjectValue(ncType, nc)
}

func kgwModelToFirewall(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	// ingress rules
	ingressRules := []attr.Value{}
	ingressRuleType := map[string]attr.Type{
		KeySource:   types.StringType,
		KeyProtocol: types.StringType,
		KeyPorts:    types.StringType,
	}
	for _, ir := range r.Firewall.Ingress {
		source := KgwDefaultValueSource
		if ir.Source != nil {
			source = *ir.Source
		}
		protocol := KgwDefaultValueProtocol
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

	// egress policy
	if r.Firewall.EgressPolicy != nil {
		d.EgressPolicy = types.StringPointerValue(r.Firewall.EgressPolicy)
	} else {
		d.EgressPolicy = types.StringValue(KgwDefaultValueEgressPolicy)
	}

	// egress rules
	egressRules := []attr.Value{}
	egressRuleType := map[string]attr.Type{
		KeyDestination: types.StringType,
		KeyProtocol:    types.StringType,
		KeyPorts:       types.StringType,
	}
	for _, er := range r.Firewall.Egress {
		destination := KgwDefaultValueDestination
		if er.Destination != nil {
			destination = *er.Destination
		}
		protocol := KgwDefaultValueProtocol
		if er.Protocol != nil {
			protocol = *er.Protocol
		}
		r := map[string]attr.Value{
			KeyDestination: types.StringValue(destination),
			KeyProtocol:    types.StringValue(protocol),
			KeyPorts:       types.StringValue(er.Ports),
		}
		object, _ := types.ObjectValue(egressRuleType, r)
		egressRules = append(egressRules, object)
	}

	if len(r.Firewall.Egress) == 0 {
		d.EgressRules = types.ListNull(types.ObjectType{AttrTypes: egressRuleType})
	} else {

		d.EgressRules, _ = types.ListValue(types.ObjectType{AttrTypes: egressRuleType}, egressRules)
	}
}

func kgwModelToNatRules(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	rules := []attr.Value{}
	ruleType := map[string]attr.Type{
		KeyDestination: types.StringType,
		KeyProtocol:    types.StringType,
		KeyPorts:       types.StringType,
	}

	// empty rules ?
	if len(r.Dnat) == 0 {
		d.NatRules = types.ListNull(types.ObjectType{AttrTypes: ruleType})
		return
	}

	for _, rule := range r.Dnat {
		protocol := KgwDefaultValueProtocol
		if rule.Protocol != nil {
			protocol = *rule.Protocol
		}
		r := map[string]attr.Value{
			KeyDestination: types.StringValue(rule.Destination),
			KeyProtocol:    types.StringValue(protocol),
			KeyPorts:       types.StringValue(rule.Ports),
		}
		object, _ := types.ObjectValue(ruleType, r)
		rules = append(rules, object)
	}
	d.NatRules, _ = types.ListValue(types.ObjectType{AttrTypes: ruleType}, rules)
}

func kgwModelToVpcPeerings(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	vpc := []attr.Value{}
	vpcType := map[string]attr.Type{
		KeySubnet: types.StringType,
		KeyPolicy: types.StringType,
		KeyIngressRules: types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					KeyProtocol: types.StringType,
					KeyPorts:    types.StringType,
				},
			},
		},
		KeyEgressRules: types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					KeyProtocol: types.StringType,
					KeyPorts:    types.StringType,
				},
			},
		},
		KeyNetworkConfig: types.ListType{
			ElemType: types.ObjectType{
				AttrTypes: map[string]attr.Type{
					KeyZone:      types.StringType,
					KeyPrivateIP: types.StringType,
				},
			},
		},
	}

	// empty peerings ?
	if len(r.VpcPeerings) == 0 {
		d.VpcPeerings = types.ListNull(types.ObjectType{AttrTypes: vpcType})
		return
	}

	fwRuleType := map[string]attr.Type{
		KeyProtocol: types.StringType,
		KeyPorts:    types.StringType,
	}

	netCfgType := map[string]attr.Type{
		KeyZone:      types.StringType,
		KeyPrivateIP: types.StringType,
	}

	for _, vp := range r.VpcPeerings {
		policy := KgwDefaultValueForwardPolicy
		if vp.Policy != nil {
			policy = *vp.Policy
		}

		// ingress rules
		ingressRules := []attr.Value{}
		for _, ir := range vp.Ingress {
			protocol := KgwDefaultValueProtocol
			if ir.Protocol != nil {
				protocol = *ir.Protocol
			}

			rule := map[string]attr.Value{
				KeyProtocol: types.StringValue(protocol),
				KeyPorts:    types.StringValue(ir.Ports),
			}
			object, _ := types.ObjectValue(fwRuleType, rule)
			ingressRules = append(ingressRules, object)
		}

		// egress rules
		egressRules := []attr.Value{}
		for _, er := range vp.Egress {
			protocol := KgwDefaultValueProtocol
			if er.Protocol != nil {
				protocol = *er.Protocol
			}

			rule := map[string]attr.Value{
				KeyProtocol: types.StringValue(protocol),
				KeyPorts:    types.StringValue(er.Ports),
			}
			object, _ := types.ObjectValue(fwRuleType, rule)
			egressRules = append(egressRules, object)
		}

		// network config
		netCfg := []attr.Value{}
		for _, cfg := range vp.Netip {
			v := map[string]attr.Value{
				KeyZone:      types.StringValue(cfg.Zone),
				KeyPrivateIP: types.StringValue(cfg.Private),
			}
			object, _ := types.ObjectValue(netCfgType, v)
			netCfg = append(netCfg, object)
		}

		r := map[string]attr.Value{
			KeySubnet: types.StringValue(vp.Subnet),
			KeyPolicy: types.StringValue(policy),
		}
		r[KeyIngressRules], _ = types.ListValue(types.ObjectType{AttrTypes: fwRuleType}, ingressRules)
		r[KeyEgressRules], _ = types.ListValue(types.ObjectType{AttrTypes: fwRuleType}, egressRules)
		r[KeyNetworkConfig], _ = types.ListValue(types.ObjectType{AttrTypes: netCfgType}, netCfg)

		object, _ := types.ObjectValue(vpcType, r)
		vpc = append(vpc, object)
	}
	d.VpcPeerings, _ = types.ListValue(types.ObjectType{AttrTypes: vpcType}, vpc)
}

func kgwModelToResource(ctx *context.Context, r *sdk.KGW, d *KgwResourceModel) {
	if r == nil {
		return
	}
	if r.Description != nil {
		d.Desc = types.StringPointerValue(r.Description)
	} else {
		d.Desc = types.StringValue("")
	}

	kgwModelToNetworkConfig(ctx, r, d)
	kgwModelToFirewall(ctx, r, d)
	kgwModelToNatRules(ctx, r, d)
	kgwModelToVpcPeerings(ctx, r, d)
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
