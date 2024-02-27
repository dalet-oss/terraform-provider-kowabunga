package provider

import (
	"context"

	"golang.org/x/exp/maps"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go/client"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	VolumeResourceName = "volume"
)

var _ resource.Resource = &VolumeResource{}
var _ resource.ResourceWithImportState = &VolumeResource{}

func NewVolumeResource() resource.Resource {
	return &VolumeResource{}
}

type VolumeResource struct {
	Data *KowabungaProviderData
}

type VolumeResourceModel struct {
	ID        types.String   `tfsdk:"id"`
	Timeouts  timeouts.Value `tfsdk:"timeouts"`
	Name      types.String   `tfsdk:"name"`
	Desc      types.String   `tfsdk:"desc"`
	Project   types.String   `tfsdk:"project"`
	Zone      types.String   `tfsdk:"zone"`
	Pool      types.String   `tfsdk:"pool"`
	Template  types.String   `tfsdk:"template"`
	Type      types.String   `tfsdk:"type"`
	Size      types.Int64    `tfsdk:"size"`
	Resizable types.Bool     `tfsdk:"resizable"`
}

func (r *VolumeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, VolumeResourceName)
}

func (r *VolumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *VolumeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *VolumeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a storage volume resource",
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
			},
			KeyType: schema.StringAttribute{
				MarkdownDescription: "The volume type (valid options: 'os', 'iso', 'raw')",
				Required:            true,
			},
			KeyTemplate: schema.StringAttribute{
				MarkdownDescription: "The template name or ID",
				Optional:            true,
			},
			KeySize: schema.Int64Attribute{
				MarkdownDescription: "The volume size (expressed in GB)",
				Required:            true,
			},
			KeyResizable: schema.BoolAttribute{
				MarkdownDescription: "Is the storage volume allowed to grow (filesystem dependant) ? (default: **false**)",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts volume from Terraform model to Kowabunga API model
func volumeResourceToModel(d *VolumeResourceModel) sdk.Volume {
	return sdk.Volume{
		Name:        d.Name.ValueString(),
		Description: d.Desc.ValueStringPointer(),
		Type:        d.Type.ValueString(),
		Size:        d.Size.ValueInt64() * HelperGbToBytes,
		Resizable:   d.Resizable.ValueBoolPointer(),
	}
}

// converts volume from Kowabunga API model to Terraform model
func volumeModelToResource(r *sdk.Volume, d *VolumeResourceModel) {
	d.Name = types.StringValue(r.Name)
	d.Desc = types.StringPointerValue(r.Description)
	d.Type = types.StringValue(r.Type)
	d.Size = types.Int64Value(r.Size / HelperGbToBytes)
	d.Resizable = types.BoolPointerValue(r.Resizable)
}

func (r *VolumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *VolumeResourceModel
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

	// create a new volume
	m := volumeResourceToModel(data)
	api := r.Data.K.ProjectAPI.CreateProjectZoneVolume(ctx, projectId, zoneId).Volume(m)
	if poolId != "" {
		api.PoolId(poolId)
	}
	if templateId != "" {
		api.TemplateId(templateId)
	}
	volume, _, err := api.Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringPointerValue(volume.Id)
	volumeModelToResource(volume, data) // read back resulting object
	tflog.Trace(ctx, "created volume resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *VolumeResourceModel
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

	volume, _, err := r.Data.K.VolumeAPI.ReadVolume(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	volumeModelToResource(volume, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *VolumeResourceModel
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

	m := volumeResourceToModel(data)
	_, _, err := r.Data.K.VolumeAPI.UpdateVolume(ctx, data.ID.ValueString()).Volume(m).Execute()
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *VolumeResourceModel
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

	_, err := r.Data.K.VolumeAPI.DeleteVolume(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
