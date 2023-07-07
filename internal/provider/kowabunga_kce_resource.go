package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/client/kce"
	"github.com/dalet-oss/kowabunga-api/client/project"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	KceResourceName = "kce"
)

var _ resource.Resource = &KceResource{}
var _ resource.ResourceWithImportState = &KceResource{}

func NewKceResource() resource.Resource {
	return &KceResource{}
}

type KceResource struct {
	Data *KowabungaProviderData
}

type KceResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Desc      types.String `tfsdk:"desc"`
	Project   types.String `tfsdk:"project"`
	Zone      types.String `tfsdk:"zone"`
	Pool      types.String `tfsdk:"pool"`
	Template  types.String `tfsdk:"template"`
	VCPUs     types.Int64  `tfsdk:"vcpus"`
	Memory    types.Int64  `tfsdk:"mem"`
	Disk      types.Int64  `tfsdk:"disk"`
	ExtraDisk types.Int64  `tfsdk:"extra_disk"`
	Public    types.Bool   `tfsdk:"public"`
	Notify    types.Bool   `tfsdk:"notify"`
	IP        types.String `tfsdk:"ip"`
}

func (r *KceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, KceResourceName)
}

func (r *KceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyIP), req, resp)
}

func (r *KceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *KceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a virtual machine KCE resource. **KCE** (stands for *Kowabunga Compute Engine*) is an seamless automated way to create virtual machine resources. It abstract the complexity of manually creating instance, volumes and network adapters resources and binding them together. It is the **RECOMMENDED** way to create and manipulate virtual machine services, unless a specific hwardware configuration is required. KCE provides 2 network adapters, a public (WAN) and a private (LAN/VPC) one, as well as up to two disks (first one for OS, optional second one for extra data).",
		Attributes: map[string]schema.Attribute{
			KeyProject: schema.StringAttribute{
				MarkdownDescription: "Associated project name or ID",
				Required:            true,
			},
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyPool: schema.StringAttribute{
				MarkdownDescription: "Associated pool name or ID (zone's default if unspecified)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			KeyTemplate: schema.StringAttribute{
				MarkdownDescription: "Associated template name or ID (zone's default pool's default if unspecified)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			KeyVCPUs: schema.Int64Attribute{
				MarkdownDescription: "The KCE number of vCPUs",
				Required:            true,
			},
			KeyMemory: schema.Int64Attribute{
				MarkdownDescription: "The KCE memory size (expressed in GB)",
				Required:            true,
			},
			KeyDisk: schema.Int64Attribute{
				MarkdownDescription: "The KCE OS disk size (expressed in GB)",
				Required:            true,
			},
			KeyExtraDisk: schema.Int64Attribute{
				MarkdownDescription: "The KCE optional data disk size (expressed in GB, disabled by default, 0 to disable)",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyPublic: schema.BoolAttribute{
				MarkdownDescription: "Should KCE be exposed over public Internet ? (default: **false**)",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			KeyNotify: schema.BoolAttribute{
				MarkdownDescription: "Whether to send email notification at creation (default: **true**)",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
			},
			KeyIP: schema.StringAttribute{
				MarkdownDescription: "IP",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts kce from Terraform model to Kowabunga API model
func kceResourceToModel(d *KceResourceModel) models.KCE {
	memSize := d.Memory.ValueInt64() * HelperGbToBytes
	diskSize := d.Disk.ValueInt64() * HelperGbToBytes
	extraDiskSize := d.ExtraDisk.ValueInt64() * HelperGbToBytes

	return models.KCE{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Vcpus:       d.VCPUs.ValueInt64Pointer(),
		Memory:      &memSize,
		Disk:        &diskSize,
		DataDisk:    extraDiskSize,
		IP:          d.IP.ValueString(),
	}
}

// converts kce from Kowabunga API model to Terraform model
func kceModelToResource(r *models.KCE, d *KceResourceModel) {
	if r == nil {
		return
	}

	memSize := *r.Memory / HelperGbToBytes
	diskSize := *r.Disk / HelperGbToBytes
	extraDiskSize := r.DataDisk / HelperGbToBytes

	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.VCPUs = types.Int64PointerValue(r.Vcpus)
	d.Memory = types.Int64Value(memSize)
	d.Disk = types.Int64Value(diskSize)
	d.ExtraDisk = types.Int64Value(extraDiskSize)
	d.IP = types.StringValue(r.IP)
}

func (r *KceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *KceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	// find parent pool (optional)
	poolId, _ := getPoolID(r.Data, data.Pool.ValueString())

	// find parent template (optional)
	templateId, _ := getTemplateID(r.Data, data.Template.ValueString())

	// create a new KCE
	cfg := kceResourceToModel(data)
	params := project.NewCreateProjectZoneKceParams().WithProjectID(projectId).WithZoneID(zoneId).WithPublic(data.Public.ValueBoolPointer()).WithNotify(data.Notify.ValueBoolPointer()).WithBody(&cfg)
	if poolId != "" {
		params = params.WithPoolID(&poolId)
	}
	if templateId != "" {
		params = params.WithTemplateID(&templateId)
	}
	obj, err := r.Data.K.Project.CreateProjectZoneKce(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	kceModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created KCE resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *KceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := kce.NewGetKCEParams().WithKceID(data.ID.ValueString())
	obj, err := r.Data.K.Kce.GetKCE(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	kceModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *KceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := kceResourceToModel(data)
	params := kce.NewUpdateKCEParams().WithKceID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Kce.UpdateKCE(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *KceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := kce.NewDeleteKCEParams().WithKceID(data.ID.ValueString())
	_, err := r.Data.K.Kce.DeleteKCE(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
