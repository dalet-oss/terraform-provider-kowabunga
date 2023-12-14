package provider

import (
	"context"

	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/project"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/volume"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

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
func volumeResourceToModel(d *VolumeResourceModel) models.Volume {
	size := d.Size.ValueInt64() * HelperGbToBytes
	return models.Volume{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Type:        d.Type.ValueStringPointer(),
		Size:        &size,
		Resizable:   d.Resizable.ValueBoolPointer(),
	}
}

// converts volume from Kowabunga API model to Terraform model
func volumeModelToResource(r *models.Volume, d *VolumeResourceModel) {
	size := *r.Size / HelperGbToBytes
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.Type = types.StringPointerValue(r.Type)
	d.Size = types.Int64Value(size)
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

	// create a new volume
	cfg := volumeResourceToModel(data)
	params := project.NewCreateProjectZoneVolumeParams().WithProjectID(projectId).WithZoneID(zoneId).WithBody(&cfg).WithTimeout(timeout)
	if poolId != "" {
		params = params.WithPoolID(&poolId)
	}
	if templateId != "" {
		params = params.WithTemplateID(&templateId)
	}
	obj, err := r.Data.K.Project.CreateProjectZoneVolume(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringValue(obj.Payload.ID)
	volumeModelToResource(obj.Payload, data) // read back resulting object
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

	params := volume.NewGetVolumeParams().WithVolumeID(data.ID.ValueString()).WithTimeout(timeout)
	obj, err := r.Data.K.Volume.GetVolume(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	volumeModelToResource(obj.Payload, data)
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

	cfg := volumeResourceToModel(data)
	params := volume.NewUpdateVolumeParams().WithVolumeID(data.ID.ValueString()).WithBody(&cfg).WithTimeout(timeout)
	_, err := r.Data.K.Volume.UpdateVolume(params, nil)
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

	params := volume.NewDeleteVolumeParams().WithVolumeID(data.ID.ValueString()).WithTimeout(timeout)
	_, err := r.Data.K.Volume.DeleteVolume(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted")
}
