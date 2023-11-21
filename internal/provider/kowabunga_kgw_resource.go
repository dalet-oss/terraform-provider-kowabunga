package provider

import (
	"context"
	"time"

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
	KgwResourceName  = "kgw"
	kgwCreateTimeout = 3 * time.Minute
	kgwDeleteTimeout = 2 * time.Minute
	kgwReadTimeout   = 1 * time.Minute
	kgwUpdateTimeout = 2 * time.Minute
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
	ID      types.String `tfsdk:"id"`
	Desc    types.String `tfsdk:"desc"`
	Project types.String `tfsdk:"project"`

	Name      types.String   `tfsdk:"name"`
	Zone      types.String   `tfsdk:"zone"`
	PublicIp  types.String   `tfsdk:"public_ip"`
	PrivateIp types.String   `tfsdk:"private_ip"`
	Nats      types.List     `tfsdk:"nats"`
	Timeouts  timeouts.Value `tfsdk:"timeouts"`
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
						"private_ip": schema.StringAttribute{
							MarkdownDescription: "Private IP where the NAT will be forwarded",
							Required:            true,
						},
						"public_ip": schema.StringAttribute{
							MarkdownDescription: "Exposed public IP used to for forward traffic. Leave empty to use the default GW interface",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"ports": schema.StringAttribute{
							MarkdownDescription: "Ports that will be forwarded. 0 Means all. Ranges Accepted",
							Required:            true,
							Validators: []validator.String{
								&stringPortValidator{},
							},
						},
					},
				},
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create:            true,
				Read:              true,
				Update:            true,
				Delete:            true,
				CreateDescription: "3m",
				ReadDescription:   "3m",
				UpdateDescription: "3m",
				DeleteDescription: "3m",
			}),
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributesWithoutName())
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
		"private_ip": types.StringType,
		"public_ip":  types.StringType,
		"ports":      types.StringType,
	}
	for _, nat := range r.Nats {
		a := map[string]attr.Value{
			"private_ip": types.StringPointerValue(nat.PrivateIP),
			"public_ip":  types.StringValue(nat.PublicIP),
			"ports":      types.StringPointerValue(nat.Ports),
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
	createTimeout, diags := data.Timeouts.Create(ctx, kgwCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	// find parent project
	projectId, err := getProjectID(r.Data, data.Project.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	// find parent zone
	zoneId, err := getZoneID(r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	cfg := kgwResourceToModel(&ctx, data)

	// create a new KGW
	params := project.NewCreateProjectZoneKgwParams().
		WithProjectID(projectId).WithZoneID(zoneId).
		WithBody(&cfg).WithTimeout(createTimeout)

	obj, err := r.Data.K.Project.CreateProjectZoneKgw(params, nil)

	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	tflog.Error(ctx, "\n\n\nON SAIT JAMAIST\n\n\n")

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
	readTimeout, diags := data.Timeouts.Read(ctx, kgwReadTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := kgw.NewGetKgwParams().WithKgwID(data.ID.ValueString()).WithTimeout(readTimeout)
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

	updateTimeout, diags := data.Timeouts.Update(ctx, kgwUpdateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := kgwResourceToModel(&ctx, data)
	params := kgw.NewUpdateKGWParams().WithKgwID(data.ID.ValueString()).WithBody(&cfg).WithTimeout(updateTimeout)
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

	deleteTimeout, diags := data.Timeouts.Delete(ctx, kgwDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := kgw.NewDeleteKGWParams().WithKgwID(data.ID.ValueString()).WithTimeout(deleteTimeout)
	_, err := r.Data.K.Kgw.DeleteKGW(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
