package provider

import (
	"context"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/kgw"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/project"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

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

	Zone      types.String `tfsdk:"zone"`
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
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
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
func kgwResourceToModel(ctx *context.Context, d *KgwResourceModel) models.KGW {
	natsModel := []*models.KGWNat{}

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
		natsModel = append(natsModel, &models.KGWNat{
			PrivateIP: natRule.PrivateIp.ValueStringPointer(),
			PublicIP:  natRule.PublicIp.ValueString(),
			Ports:     natRule.Ports.ValueStringPointer(),
		})
	}
	return models.KGW{
		Description: d.Desc.ValueString(),
		PublicIP:    d.PublicIp.ValueString(),
		PrivateIP:   d.PrivateIp.ValueString(),
		Nats:        natsModel,
	}
}

// converts kgw from Kowabunga API model to Terraform model
func kgwModelToResource(ctx *context.Context, r *models.KGW, d *KgwResourceModel) {
	if r == nil {
		return
	}
	d.Desc = types.StringValue(r.Description)
	d.PublicIp = types.StringValue(r.PublicIP)
	d.PrivateIp = types.StringValue(r.PrivateIP)

	nats := []attr.Value{}
	natType := map[string]attr.Type{
		KeyPrivateIp: types.StringType,
		KeyPublicIp:  types.StringType,
		KeyPorts:     types.StringType,
	}
	for _, nat := range r.Nats {
		a := map[string]attr.Value{
			KeyPrivateIp: types.StringPointerValue(nat.PrivateIP),
			KeyPublicIp:  types.StringValue(nat.PublicIP),
			KeyPorts:     types.StringPointerValue(nat.Ports),
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
	projectId, err := getProjectID(r.Data, data.Project.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Created")
	// find parent zone
	zoneId, err := getZoneID(r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Created")
	cfg := kgwResourceToModel(&ctx, data)

	// create a new KGW
	params := project.NewCreateProjectZoneKgwParams().
		WithProjectID(projectId).WithZoneID(zoneId).
		WithBody(&cfg).WithTimeout(timeout)

	obj, err := r.Data.K.Project.CreateProjectZoneKgw(params, nil)

	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Created")
	data.ID = types.StringValue(obj.Payload.ID)
	kgwModelToResource(&ctx, obj.Payload, data) // read back resulting object
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

	params := kgw.NewGetKgwParams().WithKgwID(data.ID.ValueString()).WithTimeout(timeout)
	obj, err := r.Data.K.Kgw.GetKgw(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	kgwModelToResource(&ctx, obj.Payload, data)
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

	cfg := kgwResourceToModel(&ctx, data)
	params := kgw.NewUpdateKGWParams().WithKgwID(data.ID.ValueString()).WithBody(&cfg).WithTimeout(timeout)
	_, err := r.Data.K.Kgw.UpdateKGW(params, nil)
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

	params := kgw.NewDeleteKGWParams().WithKgwID(data.ID.ValueString()).WithTimeout(timeout)
	_, err := r.Data.K.Kgw.DeleteKGW(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted")
}
