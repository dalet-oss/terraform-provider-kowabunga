package provider

import (
	"context"
	"sort"

	"golang.org/x/exp/maps"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go/client"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	StorageNfsResourceName          = "storage_nfs"
	StorageNfsDefaultFs             = "nfs"
	StorageNfsGaneshaApiPortDefault = 54934
)

var _ resource.Resource = &StorageNfsResource{}
var _ resource.ResourceWithImportState = &StorageNfsResource{}

func NewStorageNfsResource() resource.Resource {
	return &StorageNfsResource{}
}

type StorageNfsResource struct {
	Data *KowabungaProviderData
}

type StorageNfsResourceModel struct {
	ID       types.String   `tfsdk:"id"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
	Name     types.String   `tfsdk:"name"`
	Desc     types.String   `tfsdk:"desc"`
	Zone     types.String   `tfsdk:"zone"`
	Endpoint types.String   `tfsdk:"endpoint"`
	FS       types.String   `tfsdk:"fs"`
	Backends types.List     `tfsdk:"backends"`
	Port     types.Int64    `tfsdk:"port"`
	Default  types.Bool     `tfsdk:"default"`
}

func (r *StorageNfsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, StorageNfsResourceName)
}

func (r *StorageNfsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *StorageNfsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *StorageNfsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an NFS storage resource",
		Attributes: map[string]schema.Attribute{
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyEndpoint: schema.StringAttribute{
				MarkdownDescription: "NFS storage associated FQDN",
				Required:            true,
			},
			KeyFS: schema.StringAttribute{
				MarkdownDescription: "Underlying associated CephFS volume name (default: 'nfs')",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(StorageNfsDefaultFs),
			},
			KeyBackends: schema.ListAttribute{
				MarkdownDescription: "List of NFS Ganesha API server IP addresses",
				ElementType:         types.StringType,
				Required:            true,
			},
			KeyPort: schema.Int64Attribute{
				MarkdownDescription: "NFS Ganesha API server port (default 54934)",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
					int64validator.AtMost(65535),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Default: int64default.StaticInt64(StorageNfsGaneshaApiPortDefault),
			},
			KeyDefault: schema.BoolAttribute{
				MarkdownDescription: "Whether to set NFS storage as zone's default one (default: **false**). First NFS storage to be created is always considered as default's one.",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts NFS storage from Terraform model to Kowabunga API model
func storageNfsResourceToModel(d *StorageNfsResourceModel) sdk.StorageNFS {
	backends := []string{}
	d.Backends.ElementsAs(context.TODO(), &backends, false)
	sort.Strings(backends)

	return sdk.StorageNFS{
		Name:        d.Name.ValueString(),
		Description: d.Desc.ValueStringPointer(),
		Endpoint:    d.Endpoint.ValueString(),
		Fs:          d.FS.ValueStringPointer(),
		Backends:    backends,
		Port:        d.Port.ValueInt64Pointer(),
	}
}

// converts NFS storage from Kowabunga API model to Terraform model
func storageNfsModelToResource(r *sdk.StorageNFS, d *StorageNfsResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringValue(r.Name)
	d.Desc = types.StringPointerValue(r.Description)
	d.Endpoint = types.StringValue(r.Endpoint)
	d.FS = types.StringPointerValue(r.Fs)
	backends := []attr.Value{}
	sort.Strings(r.Backends)
	for _, b := range r.Backends {
		backends = append(backends, types.StringValue(b))
	}
	d.Backends, _ = types.ListValue(types.StringType, backends)
	d.Port = types.Int64PointerValue(r.Port)
}

func (r *StorageNfsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *StorageNfsResourceModel
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

	// find parent zone
	zoneId, err := getZoneID(ctx, r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// create a new NFS storage
	m := storageNfsResourceToModel(data)
	nfs, _, err := r.Data.K.ZoneAPI.CreateStorageNFS(ctx, zoneId).StorageNFS(m).Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// set NFS storage as default
	if data.Default.ValueBool() {
		_, err = r.Data.K.ZoneAPI.SetZoneDefaultStorageNFS(ctx, zoneId, *nfs.Id).Execute()
		if err != nil {
			errorCreateGeneric(resp, err)
			return
		}
	}

	data.ID = types.StringPointerValue(nfs.Id)
	storageNfsModelToResource(nfs, data) // read back resulting object
	tflog.Trace(ctx, "created NFS storage resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StorageNfsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *StorageNfsResourceModel
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

	nfs, _, err := r.Data.K.NfsAPI.ReadStorageNFS(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	storageNfsModelToResource(nfs, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StorageNfsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *StorageNfsResourceModel
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

	m := storageNfsResourceToModel(data)
	_, _, err := r.Data.K.NfsAPI.UpdateStorageNFS(ctx, data.ID.ValueString()).StorageNFS(m).Execute()
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StorageNfsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *StorageNfsResourceModel
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

	_, err := r.Data.K.NfsAPI.DeleteStorageNFS(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
