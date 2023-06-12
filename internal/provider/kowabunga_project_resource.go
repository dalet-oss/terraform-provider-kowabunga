package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/client/project"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	ProjectResourceName = "project"
)

var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

type ProjectResource struct {
	Data *KowabungaProviderData
}

type ProjectResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Desc      types.String `tfsdk:"desc"`
	Tags      types.List   `tfsdk:"tags"`
	Metadatas types.Map    `tfsdk:"metadatas"`
}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, ProjectResourceName)
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a project resource",
		Attributes: map[string]schema.Attribute{
			KeyTags: schema.ListAttribute{
				MarkdownDescription: "List of tags associated with the project",
				ElementType:         types.StringType,
				Required:            true,
			},
			KeyMetadatas: schema.MapAttribute{
				MarkdownDescription: "List of metadatas key/value associated with the project",
				ElementType:         types.StringType,
				Required:            true,
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts project from Terraform model to Kowabunga API model
func projectResourceToModel(d *ProjectResourceModel) models.Project {
	tags := []string{}
	d.Tags.ElementsAs(context.TODO(), &tags, false)
	metas := map[string]string{}
	d.Metadatas.ElementsAs(context.TODO(), &metas, false)
	metadatas := []*models.Metadata{}
	for k, v := range metas {
		m := models.Metadata{
			Key:   k,
			Value: v,
		}
		metadatas = append(metadatas, &m)
	}

	return models.Project{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Tags:        tags,
		Metadatas:   metadatas,
	}
}

// converts project from Kowabunga API model to Terraform model
func projectModelToResource(r *models.Project, d *ProjectResourceModel) {
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	tags := []attr.Value{}
	for _, t := range r.Tags {
		tags = append(tags, types.StringValue(t))
	}
	d.Tags, _ = types.ListValue(types.StringType, tags)
	metadatas := map[string]attr.Value{}
	for _, m := range r.Metadatas {
		metadatas[m.Key] = types.StringValue(m.Value)
	}
	d.Metadatas = basetypes.NewMapValueMust(types.StringType, metadatas)
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	// create a new project
	cfg := projectResourceToModel(data)
	params := project.NewCreateProjectParams().WithBody(&cfg)
	obj, err := r.Data.K.Project.CreateProject(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	tflog.Trace(ctx, "created project resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := project.NewGetProjectParams().WithProjectID(data.ID.ValueString())
	obj, err := r.Data.K.Project.GetProject(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	projectModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := projectResourceToModel(data)
	params := project.NewUpdateProjectParams().WithProjectID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Project.UpdateProject(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *ProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := project.NewDeleteProjectParams().WithProjectID(data.ID.ValueString())
	_, err := r.Data.K.Project.DeleteProject(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
