package provider

import (
	"context"
	"golang.org/x/exp/maps"
	"sort"

	"github.com/dalet-oss/kowabunga-api/client/instance"
	"github.com/dalet-oss/kowabunga-api/client/project"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	InstanceResourceName = "instance"
)

var _ resource.Resource = &InstanceResource{}
var _ resource.ResourceWithImportState = &InstanceResource{}

func NewInstanceResource() resource.Resource {
	return &InstanceResource{}
}

type InstanceResource struct {
	Data *KowabungaProviderData
}

type InstanceResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Desc     types.String `tfsdk:"desc"`
	Project  types.String `tfsdk:"project"`
	Zone     types.String `tfsdk:"zone"`
	VCPUs    types.Int64  `tfsdk:"vcpus"`
	Memory   types.Int64  `tfsdk:"mem"`
	Adapters types.List   `tfsdk:"adapters"`
	Volumes  types.List   `tfsdk:"volumes"`
	Notify   types.Bool   `tfsdk:"notify"`
}

func (r *InstanceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, InstanceResourceName)
}

func (r *InstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *InstanceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *InstanceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a virtual machine instance resource",
		Attributes: map[string]schema.Attribute{
			KeyProject: schema.StringAttribute{
				MarkdownDescription: "Associated project name or ID",
				Required:            true,
			},
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyVCPUs: schema.Int64Attribute{
				MarkdownDescription: "The instance number of vCPUs",
				Required:            true,
			},
			KeyMemory: schema.Int64Attribute{
				MarkdownDescription: "The instance memory size (expressed in GB)",
				Required:            true,
			},
			KeyAdapters: schema.ListAttribute{
				MarkdownDescription: "The instance list of associated network adapters",
				ElementType:         types.StringType,
				Required:            true,
			},
			KeyVolumes: schema.ListAttribute{
				MarkdownDescription: "The instance list of associated storage volumes",
				ElementType:         types.StringType,
				Required:            true,
			},
			KeyNotify: schema.BoolAttribute{
				MarkdownDescription: "Whether to send email notification at creation",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts instance from Terraform model to Kowabunga API model
func instanceResourceToModel(d *InstanceResourceModel) models.Instance {
	memSize := d.Memory.ValueInt64() * HelperGbToBytes
	adapters := []string{}
	d.Adapters.ElementsAs(context.TODO(), &adapters, false)
	volumes := []string{}
	d.Volumes.ElementsAs(context.TODO(), &volumes, false)
	sort.Strings(volumes)

	return models.Instance{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Vcpus:       d.VCPUs.ValueInt64Pointer(),
		Memory:      &memSize,
		Adapters:    adapters,
		Volumes:     volumes,
	}
}

// converts instance from Kowabunga API model to Terraform model
func instanceModelToResource(r *models.Instance, d *InstanceResourceModel) {
	memSize := *r.Memory / HelperGbToBytes
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.VCPUs = types.Int64PointerValue(r.Vcpus)
	d.Memory = types.Int64Value(memSize)
	adapters := []attr.Value{}
	for _, a := range r.Adapters {
		adapters = append(adapters, types.StringValue(a))
	}
	d.Adapters, _ = types.ListValue(types.StringType, adapters)
	volumes := []attr.Value{}
	sort.Strings(r.Volumes)
	for _, v := range r.Volumes {
		volumes = append(volumes, types.StringValue(v))
	}
	d.Volumes, _ = types.ListValue(types.StringType, volumes)
}

func (r *InstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *InstanceResourceModel
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

	// create a new instance
	cfg := instanceResourceToModel(data)
	params := project.NewCreateProjectZoneInstanceParams().WithProjectID(projectId).WithZoneID(zoneId).WithNotify(data.Notify.ValueBoolPointer()).WithBody(&cfg)
	obj, err := r.Data.K.Project.CreateProjectZoneInstance(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	instanceModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created instance resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *InstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := instance.NewGetInstanceParams().WithInstanceID(data.ID.ValueString())
	obj, err := r.Data.K.Instance.GetInstance(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	instanceModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := instanceResourceToModel(data)
	params := instance.NewUpdateInstanceParams().WithInstanceID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Instance.UpdateInstance(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *InstanceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := instance.NewDeleteInstanceParams().WithInstanceID(data.ID.ValueString())
	_, err := r.Data.K.Instance.DeleteInstance(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
