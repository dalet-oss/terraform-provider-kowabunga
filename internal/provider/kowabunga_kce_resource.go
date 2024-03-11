package provider

import (
	"context"

	"golang.org/x/exp/maps"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go/client"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
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

	KceDefaultValuePool      = ""
	KceDefaultValueTemplate  = ""
	KceDefaultValueExtraDisk = 0
	KceDefaultValuePublic    = false
	KceDefaultValueNotify    = true
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
	ID        types.String   `tfsdk:"id"`
	Timeouts  timeouts.Value `tfsdk:"timeouts"`
	Name      types.String   `tfsdk:"name"`
	Desc      types.String   `tfsdk:"desc"`
	Project   types.String   `tfsdk:"project"`
	Zone      types.String   `tfsdk:"zone"`
	Pool      types.String   `tfsdk:"pool"`
	Template  types.String   `tfsdk:"template"`
	VCPUs     types.Int64    `tfsdk:"vcpus"`
	Memory    types.Int64    `tfsdk:"mem"`
	Disk      types.Int64    `tfsdk:"disk"`
	ExtraDisk types.Int64    `tfsdk:"extra_disk"`
	Public    types.Bool     `tfsdk:"public"`
	Notify    types.Bool     `tfsdk:"notify"`
	IP        types.String   `tfsdk:"ip"`
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
				MarkdownDescription: "Associated storage pool name or ID (zone's default if unspecified)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(KceDefaultValuePool),
			},
			KeyTemplate: schema.StringAttribute{
				MarkdownDescription: "Associated template name or ID (zone's default storage pool's default if unspecified)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(KceDefaultValueTemplate),
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
				Default:             int64default.StaticInt64(KceDefaultValueExtraDisk),
			},
			KeyPublic: schema.BoolAttribute{
				MarkdownDescription: "Should KCE be exposed over public Internet ? (default: **false**)",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(KceDefaultValuePublic),
			},
			KeyNotify: schema.BoolAttribute{
				MarkdownDescription: "Whether to send email notification at creation (default: **true**)",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(KceDefaultValueNotify),
			},
			KeyIP: schema.StringAttribute{
				MarkdownDescription: "IP (read-only)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts kce from Terraform model to Kowabunga API model
func kceResourceToModel(d *KceResourceModel) sdk.KCE {
	memSize := d.Memory.ValueInt64() * HelperGbToBytes
	diskSize := d.Disk.ValueInt64() * HelperGbToBytes
	extraDiskSize := d.ExtraDisk.ValueInt64() * HelperGbToBytes

	return sdk.KCE{
		Name:        d.Name.ValueString(),
		Description: d.Desc.ValueStringPointer(),
		Vcpus:       d.VCPUs.ValueInt64(),
		Memory:      memSize,
		Disk:        diskSize,
		DataDisk:    &extraDiskSize,
		Ip:          d.IP.ValueStringPointer(),
	}
}

// converts kce from Kowabunga API model to Terraform model
func kceModelToResource(r *sdk.KCE, d *KceResourceModel) {
	if r == nil {
		return
	}

	memSize := r.Memory / HelperGbToBytes
	diskSize := r.Disk / HelperGbToBytes
	var extraDiskSize int64 = 0
	if r.DataDisk != nil {
		extraDiskSize = *r.DataDisk / HelperGbToBytes
	}

	d.Name = types.StringValue(r.Name)
	if r.Description != nil {
		d.Desc = types.StringPointerValue(r.Description)
	} else {
		d.Desc = types.StringValue("")
	}
	d.VCPUs = types.Int64Value(r.Vcpus)
	d.Memory = types.Int64Value(memSize)
	d.Disk = types.Int64Value(diskSize)
	d.ExtraDisk = types.Int64Value(extraDiskSize)
	if r.Ip != nil {
		d.IP = types.StringPointerValue(r.Ip)
	} else {
		d.IP = types.StringValue("")
	}
}

func (r *KceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *KceResourceModel
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
	// find parent zone
	zoneId, err := getZoneID(ctx, r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// find parent pool (optional)
	poolId, _ := getPoolID(ctx, r.Data, data.Pool.ValueString())

	// find parent template (optional)
	templateId, _ := getTemplateID(ctx, r.Data, data.Template.ValueString())

	// create a new KCE
	m := kceResourceToModel(data)
	api := r.Data.K.ProjectAPI.CreateProjectZoneKCE(ctx, projectId, zoneId).KCE(m).Public(data.Public.ValueBool()).Notify(data.Notify.ValueBool())
	if poolId != "" {
		api = api.PoolId(poolId)
	}
	if templateId != "" {
		api = api.TemplateId(templateId)
	}
	kce, _, err := api.Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringPointerValue(kce.Id)
	kceModelToResource(kce, data) // read back resulting object
	tflog.Trace(ctx, "created KCE resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *KceResourceModel
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

	kce, _, err := r.Data.K.KceAPI.ReadKCE(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	kceModelToResource(kce, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *KceResourceModel
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

	m := kceResourceToModel(data)
	_, _, err := r.Data.K.KceAPI.UpdateKCE(ctx, data.ID.ValueString()).KCE(m).Execute()
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
	timeout, diags := data.Timeouts.Delete(ctx, DefaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	_, err := r.Data.K.KceAPI.DeleteKCE(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
