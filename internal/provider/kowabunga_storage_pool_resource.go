package provider

import (
	"context"

	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/pool"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/zone"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	StoragePoolResourceName = "storage_pool"
)

var _ resource.Resource = &StoragePoolResource{}
var _ resource.ResourceWithImportState = &StoragePoolResource{}

func NewStoragePoolResource() resource.Resource {
	return &StoragePoolResource{}
}

type StoragePoolResource struct {
	Data *KowabungaProviderData
}

type StoragePoolResourceModel struct {
	ID       types.String   `tfsdk:"id"`
	Timeouts timeouts.Value `tfsdk:"timeouts"`
	Name     types.String   `tfsdk:"name"`
	Desc     types.String   `tfsdk:"desc"`
	Zone     types.String   `tfsdk:"zone"`
	Type     types.String   `tfsdk:"type"`
	Host     types.String   `tfsdk:"host"`
	Pool     types.String   `tfsdk:"pool"`
	Address  types.String   `tfsdk:"address"`
	Port     types.Int64    `tfsdk:"port"`
	Secret   types.String   `tfsdk:"secret"`
	Price    types.Int64    `tfsdk:"price"`
	Currency types.String   `tfsdk:"currency"`
	Default  types.Bool     `tfsdk:"default"`
}

func (r *StoragePoolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, StoragePoolResourceName)
}

func (r *StoragePoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *StoragePoolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *StoragePoolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a storage pool resource",
		Attributes: map[string]schema.Attribute{
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyType: schema.StringAttribute{
				MarkdownDescription: "Storage pool type ('local' or 'rbd', defaults to 'rbd')",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyHost: schema.StringAttribute{
				MarkdownDescription: "Host to bind the storage pool to (default: none)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			KeyPool: schema.StringAttribute{
				MarkdownDescription: "Ceph RBD pool name",
				Required:            true,
			},
			KeyAddress: schema.StringAttribute{
				MarkdownDescription: "Ceph RBD monitor address or hostname",
				Optional:            true,
			},
			KeyPort: schema.Int64Attribute{
				MarkdownDescription: "Ceph RBD monitor port number",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
					int64validator.AtMost(65535),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			KeySecret: schema.StringAttribute{
				MarkdownDescription: "CephX client authentication UUID",
				Optional:            true,
				Sensitive:           true,
			},
			KeyPrice: schema.Int64Attribute{
				MarkdownDescription: "libvirt host monthly price value (default: 0)",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyCurrency: schema.StringAttribute{
				MarkdownDescription: "libvirt host monthly price currency (default: **EUR**)",
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString("EUR"),
			},
			KeyDefault: schema.BoolAttribute{
				MarkdownDescription: "Whether to set pool as zone's default one (default: **false**). First pool to be created is always considered as default's one.",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts storage pool from Terraform model to Kowabunga API model
func storagePoolResourceToModel(d *StoragePoolResourceModel) models.StoragePool {
	cost := models.Cost{
		Price:    d.Price.ValueInt64Pointer(),
		Currency: d.Currency.ValueStringPointer(),
	}
	return models.StoragePool{
		Name:           d.Name.ValueStringPointer(),
		Description:    d.Desc.ValueString(),
		Type:           d.Type.ValueStringPointer(),
		Pool:           d.Pool.ValueStringPointer(),
		CephAddress:    d.Address.ValueStringPointer(),
		CephPort:       d.Port.ValueInt64Pointer(),
		CephSecretUUID: d.Secret.ValueString(),
		Cost:           &cost,
	}
}

// converts storage pool from Kowabunga API model to Terraform model
func storagePoolModelToResource(r *models.StoragePool, d *StoragePoolResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.Type = types.StringPointerValue(r.Type)
	d.Pool = types.StringPointerValue(r.Pool)
	d.Address = types.StringPointerValue(r.CephAddress)
	d.Port = types.Int64PointerValue(r.CephPort)
	d.Secret = types.StringValue(r.CephSecretUUID)
	if r.Cost != nil {
		d.Price = types.Int64PointerValue(r.Cost.Price)
		d.Currency = types.StringPointerValue(r.Cost.Currency)
	}
}

func (r *StoragePoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *StoragePoolResourceModel
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
	zoneId, err := getZoneID(r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// find parent template (optional)
	hostId, _ := getHostID(r.Data, data.Host.ValueString())

	// create a new storage pool
	cfg := storagePoolResourceToModel(data)
	params := zone.NewCreatePoolParams().WithZoneID(zoneId).WithBody(&cfg).WithTimeout(timeout)
	if hostId != "" {
		params = params.WithHostID(&hostId)
	}
	obj, err := r.Data.K.Zone.CreatePool(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Created")
	// set storage pool as default
	if data.Default.ValueBool() {
		params2 := zone.NewUpdateZoneDefaultPoolParams().WithZoneID(zoneId).WithPoolID(obj.Payload.ID)
		_, err = r.Data.K.Zone.UpdateZoneDefaultPool(params2, nil)
		if err != nil {
			errorCreateGeneric(resp, err)
			return
		}
	}

	data.ID = types.StringValue(obj.Payload.ID)
	storagePoolModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created storage pool resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StoragePoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *StoragePoolResourceModel
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

	params := pool.NewGetPoolParams().WithPoolID(data.ID.ValueString()).WithTimeout(timeout)
	obj, err := r.Data.K.Pool.GetPool(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	storagePoolModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StoragePoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *StoragePoolResourceModel
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

	cfg := storagePoolResourceToModel(data)
	params := pool.NewUpdatePoolParams().WithPoolID(data.ID.ValueString()).WithBody(&cfg).WithTimeout(timeout)
	_, err := r.Data.K.Pool.UpdatePool(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *StoragePoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *StoragePoolResourceModel
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

	params := pool.NewDeletePoolParams().WithPoolID(data.ID.ValueString()).WithTimeout(timeout)
	_, err := r.Data.K.Pool.DeletePool(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted")
}
