package provider

import (
	"context"

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

	Region    types.String `tfsdk:"region"`
	PublicIp  types.String `tfsdk:"public_ip"`
	PrivateIp types.String `tfsdk:"private_ip"`
	Nats      types.List   `tfsdk:"nats"`
}

type KgwNatRule struct {
	PublicIp  types.String `tfsdk:"public_ip"`
	PrivateIp types.String `tfsdk:"private_ip"`
	Ports     types.String `tfsdk:"ports"`
}

func (r *KgwResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, KgwResourceName)
}

func (r *KgwResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyPrivateIp), req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyPublicIp), req, resp)
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
			KeyPublicIp: schema.StringAttribute{
				MarkdownDescription: "The KGW default Public IP (read-only)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyPrivateIp: schema.StringAttribute{
				MarkdownDescription: "The KGW Private IP (read-only)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributesWithoutName(&ctx))
}

// converts kgw from Terraform model to Kowabunga API model
func kgwResourceToModel(ctx *context.Context, d *KgwResourceModel) sdk.KGW {
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
	return sdk.KGW{
		Description: d.Desc.ValueStringPointer(),
		PublicIp:    d.PublicIp.ValueStringPointer(),
		PrivateIp:   d.PrivateIp.ValueStringPointer(),
		Nats:        natsModel,
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
	if r.PublicIp != nil {
		d.PublicIp = types.StringPointerValue(r.PublicIp)
	} else {
		d.PublicIp = types.StringValue("")
	}
	if r.PrivateIp != nil {
		d.PrivateIp = types.StringPointerValue(r.PrivateIp)
	} else {
		d.PrivateIp = types.StringValue("")
	}

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
