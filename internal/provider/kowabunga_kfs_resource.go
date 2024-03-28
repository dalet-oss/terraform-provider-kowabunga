package provider

import (
	"context"

	"golang.org/x/exp/maps"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go/client"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	KfsResourceName = "kfs"

	KfsDefaultValueNfs        = ""
	KfsDefaultValueAccessType = "RW"
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
	ID        types.String   `tfsdk:"id"`
	Timeouts  timeouts.Value `tfsdk:"timeouts"`
	Name      types.String   `tfsdk:"name"`
	Desc      types.String   `tfsdk:"desc"`
	Project   types.String   `tfsdk:"project"`
	Region    types.String   `tfsdk:"region"`
	Nfs       types.String   `tfsdk:"nfs"`
	Access    types.String   `tfsdk:"access_type"`
	Protocols types.List     `tfsdk:"protocols"`
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
			KeyRegion: schema.StringAttribute{
				MarkdownDescription: "Associated region name or ID",
				Required:            true,
			},
			KeyNfs: schema.StringAttribute{
				MarkdownDescription: "Associated NFS storage name or ID (zone's default if unspecified)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(KfsDefaultValueNfs),
			},
			KeyAccessType: schema.StringAttribute{
				MarkdownDescription: "KFS' access type. Allowed values: 'RW' or 'RO'. Defaults to RW.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(KfsDefaultValueAccessType),
			},
			KeyProtocols: schema.ListAttribute{
				MarkdownDescription: "KFS's requested NFS protocols versions (defaults to NFSv3 and NFSv4))",
				ElementType:         types.Int64Type,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(protocols),
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
func kfsResourceToModel(d *KfsResourceModel) sdk.KFS {
	protocols64 := []int64{}
	d.Protocols.ElementsAs(context.TODO(), &protocols64, false)
	protocols32 := []int32{}
	for _, p := range protocols64 {
		protocols32 = append(protocols32, int32(p))
	}

	return sdk.KFS{
		Name:        d.Name.ValueString(),
		Description: d.Desc.ValueStringPointer(),
		Access:      d.Access.ValueStringPointer(),
		Protocols:   protocols32,
		Endpoint:    d.Endpoint.ValueStringPointer(),
	}
}

// converts kfs from Kowabunga API model to Terraform model
func kfsModelToResource(r *sdk.KFS, d *KfsResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringValue(r.Name)
	if r.Description != nil {
		d.Desc = types.StringPointerValue(r.Description)
	} else {
		d.Desc = types.StringValue("")
	}
	if r.Access != nil {
		d.Access = types.StringPointerValue(r.Access)
	} else {
		d.Access = types.StringValue(KfsDefaultValueAccessType)
	}
	protocols := []attr.Value{}
	for _, p := range r.Protocols {
		protocols = append(protocols, types.Int64Value(int64(p)))
	}
	d.Protocols, _ = types.ListValue(types.Int64Type, protocols)
	if r.Endpoint != nil {
		d.Endpoint = types.StringPointerValue(r.Endpoint)
	} else {
		d.Endpoint = types.StringValue("")
	}
}

func (r *KfsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *KfsResourceModel
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
	regionId, err := getRegionID(ctx, r.Data, data.Region.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// find parent NFS storage (optional)
	nfsId, _ := getNfsID(ctx, r.Data, data.Nfs.ValueString())

	// create a new KFS
	m := kfsResourceToModel(data)
	api := r.Data.K.ProjectAPI.CreateProjectRegionKFS(ctx, projectId, regionId).KFS(m)
	if nfsId != "" {
		api = api.NfsId(nfsId)
	}
	kfs, _, err := api.Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringPointerValue(kfs.Id)
	kfsModelToResource(kfs, data) // read back resulting object
	tflog.Trace(ctx, "created KFS resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KfsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *KfsResourceModel
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

	kfs, _, err := r.Data.K.KfsAPI.ReadKFS(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	kfsModelToResource(kfs, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KfsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *KfsResourceModel
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

	m := kfsResourceToModel(data)
	_, _, err := r.Data.K.KfsAPI.UpdateKFS(ctx, data.ID.ValueString()).KFS(m).Execute()
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
	timeout, diags := data.Timeouts.Delete(ctx, DefaultDeleteTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	_, err := r.Data.K.KfsAPI.DeleteKFS(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
