package provider

import (
	"context"

	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/kfs"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/project"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	KfsResourceName = "kfs"
)

var _ resource.Resource = &KfsResource{}
var _ resource.ResourceWithImportState = &KfsResource{}

func NewKfsResource() resource.Resource {
	return &KfsResource{}
}

type KfsResource struct {
	Data *KowabungaProviderData
}

type KfsResourceModel struct {
	//anonymous field
	ResourceBaseModel

	Name      types.String `tfsdk:"name"`
	Desc      types.String `tfsdk:"desc"`
	Project   types.String `tfsdk:"project"`
	Zone      types.String `tfsdk:"zone"`
	Nfs       types.String `tfsdk:"nfs"`
	Access    types.String `tfsdk:"access_type"`
	Protocols types.List   `tfsdk:"protocols"`
	Notify    types.Bool   `tfsdk:"notify"`
	// read-only
	Endpoint types.String `tfsdk:"endpoint"`
}

func (r *KfsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, KfsResourceName)
}

func (r *KfsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyIP), req, resp)
}

func (r *KfsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *KfsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	prot := []attr.Value{
		types.Int64Value(3),
		types.Int64Value(4),
	}
	protocols, _ := types.ListValue(types.Int64Type, prot)

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a KFS distributed network storage resource. **KFS** (stands for *Kowabunga File System*) provides an elastic NFS-compatible endpoint.",
		Attributes: map[string]schema.Attribute{
			KeyProject: schema.StringAttribute{
				MarkdownDescription: "Associated project name or ID",
				Required:            true,
			},
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyNfs: schema.StringAttribute{
				MarkdownDescription: "Associated NFS storage name or ID (zone's default if unspecified)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			KeyAccessType: schema.StringAttribute{
				MarkdownDescription: "KFS' access type. Allowed values: 'RW' or 'RO'. Defaults to RW.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("RW"),
			},
			KeyProtocols: schema.ListAttribute{
				MarkdownDescription: "KFS's requested NFS protocols versions (defaults to NFSv3 and NFSv4))",
				ElementType:         types.Int64Type,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(protocols),
			},
			KeyNotify: schema.BoolAttribute{
				MarkdownDescription: "Whether to send email notification at creation (default: **true**)",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
			},
			KeyEndpoint: schema.StringAttribute{
				MarkdownDescription: "NFS Endoint (read-only)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts kfs from Terraform model to Kowabunga API model
func kfsResourceToModel(d *KfsResourceModel) models.KFS {
	protocols := []int64{}
	d.Protocols.ElementsAs(context.TODO(), &protocols, false)

	return models.KFS{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Access:      d.Access.ValueStringPointer(),
		Protocols:   protocols,
		Endpoint:    d.Endpoint.ValueString(),
	}
}

// converts kfs from Kowabunga API model to Terraform model
func kfsModelToResource(r *models.KFS, d *KfsResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.Access = types.StringPointerValue(r.Access)
	protocols := []attr.Value{}
	for _, p := range r.Protocols {
		protocols = append(protocols, types.Int64Value(p))
	}
	d.Protocols, _ = types.ListValue(types.Int64Type, protocols)
	d.Endpoint = types.StringValue(r.Endpoint)
}

func (r *KfsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *KfsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, createTimeout, cancel := data.SetCreateTimeout(ctx, resp, DefaultCreateTimeout)
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

	// find parent NFS storage (optional)
	nfsId, _ := getNfsID(r.Data, data.Nfs.ValueString())

	// create a new KFS
	cfg := kfsResourceToModel(data)
	params := project.NewCreateProjectZoneKfsParams().WithProjectID(projectId).WithZoneID(zoneId).WithNotify(data.Notify.ValueBoolPointer()).WithBody(&cfg).WithTimeout(createTimeout)
	if nfsId != "" {
		params = params.WithNfsID(&nfsId)
	}
	obj, err := r.Data.K.Project.CreateProjectZoneKfs(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	kfsModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created KFS resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KfsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *KfsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, readTimeout, cancel := data.SetReadTimeout(ctx, resp, DefaultReadTimeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := kfs.NewGetKFSParams().WithKfsID(data.ID.ValueString()).WithTimeout(readTimeout)
	obj, err := r.Data.K.Kfs.GetKFS(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	kfsModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KfsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *KfsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, updateTimeout, cancel := data.SetUpdateTimeout(ctx, resp, DefaultUpdateTimeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := kfsResourceToModel(data)
	params := kfs.NewUpdateKFSParams().WithKfsID(data.ID.ValueString()).WithBody(&cfg).WithTimeout(updateTimeout)
	_, err := r.Data.K.Kfs.UpdateKFS(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KfsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *KfsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, deleteTimeout, cancel := data.SetDeleteTimeout(ctx, resp, DefaultDeleteTimeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := kfs.NewDeleteKFSParams().WithKfsID(data.ID.ValueString()).WithTimeout(deleteTimeout)
	_, err := r.Data.K.Kfs.DeleteKFS(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
