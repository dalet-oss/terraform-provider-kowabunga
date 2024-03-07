package provider

import (
	"context"

	"golang.org/x/exp/maps"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go/client"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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
	ID               types.String   `tfsdk:"id"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
	Name             types.String   `tfsdk:"name"`
	Desc             types.String   `tfsdk:"desc"`
	Zone             types.String   `tfsdk:"zone"`
	Protocol         types.String   `tfsdk:"protocol"`
	Address          types.String   `tfsdk:"address"`
	Port             types.Int64    `tfsdk:"port"`
	TlsKey           types.String   `tfsdk:"key"`
	TlsCert          types.String   `tfsdk:"cert"`
	TlsCA            types.String   `tfsdk:"ca"`
	CpuPrice         types.Int64    `tfsdk:"cpu_price"`
	MemoryPrice      types.Int64    `tfsdk:"memory_price"`
	Currency         types.String   `tfsdk:"currency"`
	CpuOvercommit    types.Int64    `tfsdk:"cpu_overcommit"`
	MemoryOvercommit types.Int64    `tfsdk:"memory_overcommit"`
	Agents           types.List     `tfsdk:"agents"`
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
				MarkdownDescription: "libvirt host API port number (defaults to 0, i.e. auto-detected)",
				Computed:            true,
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
					int64validator.AtMost(65535),
				},
				Default: int64default.StaticInt64(0),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			KeyTlsKey: schema.StringAttribute{
				MarkdownDescription: "libvirt host API TLS private key (default: none)",
				Optional:            true,
				Sensitive:           true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyTlsCert: schema.StringAttribute{
				MarkdownDescription: "libvirt host API TLS certificate (default: none)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyTlsCA: schema.StringAttribute{
				MarkdownDescription: "libvirt host API TLS CA (default: none)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyCpuPrice: schema.Int64Attribute{
				MarkdownDescription: "libvirt host monthly CPU price value (default: 0)",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyMemoryPrice: schema.Int64Attribute{
				MarkdownDescription: "libvirt host monthly Memory price value (default: 0)",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyCurrency: schema.StringAttribute{
				MarkdownDescription: "libvirt host monthly price currency (default: **EUR**)",
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString("EUR"),
			},
			KeyCpuOvercommit: schema.Int64Attribute{
				MarkdownDescription: "libvirt host CPU over-commit factor (default: 3)",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(3),
			},
			KeyMemoryOvercommit: schema.Int64Attribute{
				MarkdownDescription: "libvirt host Memory over-commit factor (default: 2)",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(2),
			},
			KeyAgents: schema.ListAttribute{
				MarkdownDescription: "The list of Kowabunga remote agents to be associated with the host",
				ElementType:         types.StringType,
				Required:            true,
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts host from Terraform model to Kowabunga API model
func hostResourceToModel(d *HostResourceModel) sdk.Host {
	agents := []string{}
	d.Agents.ElementsAs(context.TODO(), &agents, false)

	return sdk.Host{
		Name:        d.Name.ValueString(),
		Description: d.Desc.ValueStringPointer(),
		Protocol:    d.Protocol.ValueString(),
		Address:     d.Address.ValueString(),
		Port:        d.Port.ValueInt64Pointer(),
		Tls: sdk.HostTLS{
			Key:  d.TlsKey.ValueString(),
			Cert: d.TlsCert.ValueString(),
			Ca:   d.TlsCA.ValueString(),
		},
		CpuCost: sdk.Cost{
			Price:    float32(d.CpuPrice.ValueInt64()),
			Currency: d.Currency.ValueString(),
		},
		MemoryCost: sdk.Cost{
			Price:    float32(d.MemoryPrice.ValueInt64()),
			Currency: d.Currency.ValueString(),
		},
		OvercommitCpuRatio:    d.CpuOvercommit.ValueInt64Pointer(),
		OvercommitMemoryRatio: d.MemoryOvercommit.ValueInt64Pointer(),
		Agents:                agents,
	}
}

// converts host from Kowabunga API model to Terraform model
func hostModelToResource(r *sdk.Host, d *HostResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringValue(r.Name)
	d.Desc = types.StringPointerValue(r.Description)
	d.Protocol = types.StringValue(r.Protocol)
	d.Address = types.StringValue(r.Address)
	d.Port = types.Int64PointerValue(r.Port)
	d.CpuPrice = types.Int64Value(int64(r.CpuCost.Price))
	d.Currency = types.StringValue(r.CpuCost.Currency)
	d.MemoryPrice = types.Int64Value(int64(r.MemoryCost.Price))
	d.TlsKey = types.StringValue(r.Tls.Key)
	d.TlsCert = types.StringValue(r.Tls.Cert)
	d.TlsCA = types.StringValue(r.Tls.Ca)
	d.CpuOvercommit = types.Int64PointerValue(r.OvercommitCpuRatio)
	d.MemoryOvercommit = types.Int64PointerValue(r.OvercommitMemoryRatio)
	agents := []attr.Value{}
	for _, a := range r.Agents {
		agents = append(agents, types.StringValue(a))
	}
	d.Agents, _ = types.ListValue(types.StringType, agents)
}

func (r *HostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *HostResourceModel
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

	// find parent zone
	zoneId, err := getZoneID(ctx, r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// create a new host
	m := hostResourceToModel(data)
	host, _, err := r.Data.K.ZoneAPI.CreateHost(ctx, zoneId).Host(m).Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringPointerValue(host.Id)
	hostModelToResource(host, data) // read back resulting object
	tflog.Trace(ctx, "created host resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *HostResourceModel
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

	host, _, err := r.Data.K.HostAPI.ReadHost(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	hostModelToResource(host, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *HostResourceModel
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

	m := hostResourceToModel(data)
	_, _, err := r.Data.K.HostAPI.UpdateHost(ctx, data.ID.ValueString()).Host(m).Execute()
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
	timeout, diags := data.Timeouts.Delete(ctx, DefaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	_, err := r.Data.K.HostAPI.DeleteHost(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
