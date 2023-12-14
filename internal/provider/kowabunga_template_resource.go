package provider

import (
	"context"

	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/pool"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/template"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	TemplateResourceName = "template"
)

var _ resource.Resource = &TemplateResource{}
var _ resource.ResourceWithImportState = &TemplateResource{}

func NewTemplateResource() resource.Resource {
	return &TemplateResource{}
}

type TemplateResource struct {
	Data *KowabungaProviderData
}

type TemplateResourceModel struct {
	ID       types.String   `tfsdk:"id"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
	Name     types.String   `tfsdk:"name"`
	Desc     types.String   `tfsdk:"desc"`
	Pool     types.String   `tfsdk:"pool"`
	Type     types.String   `tfsdk:"type"`
	OS       types.String   `tfsdk:"os"`
	Default  types.Bool     `tfsdk:"default"`
}

func (r *TemplateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, TemplateResourceName)
}

func (r *TemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *TemplateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *TemplateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a storage pool's template resource",
		Attributes: map[string]schema.Attribute{
			KeyPool: schema.StringAttribute{
				MarkdownDescription: "Associated storage pool name or ID",
				Required:            true,
			},
			KeyType: schema.StringAttribute{
				MarkdownDescription: "The template type (valid options: 'os', 'raw'). Defaults to **os**.",
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString(models.TemplateTypeOs),
			},
			KeyOS: schema.StringAttribute{
				MarkdownDescription: "The template type (valid options: 'linux', 'windows'). Defaults to **linux**.",
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString(models.TemplateOsLinux),
			},
			KeyDefault: schema.BoolAttribute{
				MarkdownDescription: "Whether to set template as zone's default one (default: **false**). The first template to be created is always considered as default.",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts template from Terraform model to Kowabunga API model
func templateResourceToModel(d *TemplateResourceModel) models.Template {
	return models.Template{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Type:        d.Type.ValueStringPointer(),
		Os:          d.OS.ValueStringPointer(),
	}
}

// converts template from Kowabunga API model to Terraform model
func templateModelToResource(r *models.Template, d *TemplateResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.Type = types.StringPointerValue(r.Type)
	d.OS = types.StringPointerValue(r.Os)
}

func (r *TemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *TemplateResourceModel
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

	// find parent pool
	poolId, err := getPoolID(r.Data, data.Pool.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// create a new template
	cfg := templateResourceToModel(data)
	params := pool.NewCreateTemplateParams().WithPoolID(poolId).WithBody(&cfg).WithTimeout(timeout)
	obj, err := r.Data.K.Pool.CreateTemplate(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// set template as default
	if data.Default.ValueBool() {
		params2 := pool.NewUpdatePoolDefaultTemplateParams().WithPoolID(poolId).WithTemplateID(obj.Payload.ID)
		_, err = r.Data.K.Pool.UpdatePoolDefaultTemplate(params2, nil)
		if err != nil {
			errorCreateGeneric(resp, err)
			return
		}
	}

	data.ID = types.StringValue(obj.Payload.ID)
	templateModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created template resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *TemplateResourceModel
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

	params := template.NewGetTemplateParams().WithTemplateID(data.ID.ValueString()).WithTimeout(timeout)
	obj, err := r.Data.K.Template.GetTemplate(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	templateModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *TemplateResourceModel
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

	cfg := templateResourceToModel(data)
	params := template.NewUpdateTemplateParams().WithTemplateID(data.ID.ValueString()).WithBody(&cfg).WithTimeout(timeout)
	_, err := r.Data.K.Template.UpdateTemplate(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *TemplateResourceModel
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

	params := template.NewDeleteTemplateParams().WithTemplateID(data.ID.ValueString()).WithTimeout(timeout)
	_, err := r.Data.K.Template.DeleteTemplate(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted")
}
