package provider

import (
	"context"

	"github.com/dalet-oss/kowabunga-api/client/region"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	RegionResourceName = "region"
)

var _ resource.Resource = &RegionResource{}
var _ resource.ResourceWithImportState = &RegionResource{}

func NewRegionResource() resource.Resource {
	return &RegionResource{}
}

type RegionResource struct {
	Data *KowabungaProviderData
}

type RegionResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Desc types.String `tfsdk:"desc"`
}

func (r *RegionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, RegionResourceName)
}

func (r *RegionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *RegionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *RegionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a region resource",
		Attributes:          resourceAttributes(),
	}
}

// converts region from Terraform model to Kowabunga API model
func regionResourceToModel(d *RegionResourceModel) models.Region {
	return models.Region{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
	}
}

// converts region from Kowabunga API model to Terraform model
func regionModelToResource(r *models.Region, d *RegionResourceModel) {
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
}

func (r *RegionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *RegionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := regionResourceToModel(data)
	params := region.NewCreateRegionParams().WithBody(&cfg)
	obj, err := r.Data.K.Region.CreateRegion(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	regionModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created region resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *RegionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := region.NewGetRegionParams().WithRegionID(data.ID.ValueString())
	obj, err := r.Data.K.Region.GetRegion(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	regionModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *RegionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := regionResourceToModel(data)
	params := region.NewUpdateRegionParams().WithRegionID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Region.UpdateRegion(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *RegionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := region.NewDeleteRegionParams().WithRegionID(data.ID.ValueString())
	_, err := r.Data.K.Region.DeleteRegion(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
