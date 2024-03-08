package provider

import (
	"context"

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	StoragePoolResourceName = "storage_pool"

	StoragePoolDefaultValueType     = "rbd"
	StoragePoolDefaultValueHost     = ""
	StoragePoolDefaultValuePrice    = 0
	StoragePoolDefaultValueCurrency = "EUR"
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
	Agents   types.List     `tfsdk:"agents"`
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
				Default: stringdefault.StaticString(StoragePoolDefaultValueType),
			},
			KeyHost: schema.StringAttribute{
				MarkdownDescription: "Host to bind the storage pool to (default: none)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(StoragePoolDefaultValueHost),
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
				Default:             int64default.StaticInt64(StoragePoolDefaultValuePrice),
			},
			KeyCurrency: schema.StringAttribute{
				MarkdownDescription: "libvirt host monthly price currency (default: **EUR**)",
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString(StoragePoolDefaultValueCurrency),
			},
			KeyDefault: schema.BoolAttribute{
				MarkdownDescription: "Whether to set pool as zone's default one (default: **false**). First pool to be created is always considered as default's one.",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
			KeyAgents: schema.ListAttribute{
				MarkdownDescription: "The list of Kowabunga remote agents to be associated with the storage pool",
				ElementType:         types.StringType,
				Required:            true,
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts storage pool from Terraform model to Kowabunga API model
func storagePoolResourceToModel(d *StoragePoolResourceModel) sdk.StoragePool {
	cost := sdk.Cost{
		Price:    float32(d.Price.ValueInt64()),
		Currency: d.Currency.ValueString(),
	}

	agents := []string{}
	d.Agents.ElementsAs(context.TODO(), &agents, false)

	return sdk.StoragePool{
		Name:           d.Name.ValueString(),
		Description:    d.Desc.ValueStringPointer(),
		Type:           d.Type.ValueStringPointer(),
		Pool:           d.Pool.ValueString(),
		CephAddress:    d.Address.ValueStringPointer(),
		CephPort:       d.Port.ValueInt64Pointer(),
		CephSecretUuid: d.Secret.ValueStringPointer(),
		Cost:           cost,
		Agents:         agents,
	}
}

// converts storage pool from Kowabunga API model to Terraform model
func storagePoolModelToResource(r *sdk.StoragePool, d *StoragePoolResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringValue(r.Name)
	if r.Description != nil {
		d.Desc = types.StringPointerValue(r.Description)
	} else {
		d.Desc = types.StringValue("")
	}
	if r.Type != nil {
		d.Type = types.StringPointerValue(r.Type)
	} else {
		d.Type = types.StringValue(StoragePoolDefaultValueType)
	}
	d.Pool = types.StringValue(r.Pool)
	if r.CephAddress != nil {
		d.Address = types.StringPointerValue(r.CephAddress)
	} else {
		d.Address = types.StringValue("")
	}
	if r.CephPort != nil {
		d.Port = types.Int64PointerValue(r.CephPort)
	} else {
		d.Port = types.Int64Value(0)
	}
	if r.CephSecretUuid != nil {
		d.Secret = types.StringPointerValue(r.CephSecretUuid)
	} else {
		d.Secret = types.StringValue("")
	}
	d.Price = types.Int64Value(int64(r.Cost.Price))
	d.Currency = types.StringValue(r.Cost.Currency)
	agents := []attr.Value{}
	for _, a := range r.Agents {
		agents = append(agents, types.StringValue(a))
	}
	d.Agents, _ = types.ListValue(types.StringType, agents)
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
	zoneId, err := getZoneID(ctx, r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// find parent template (optional)
	hostId, _ := getHostID(ctx, r.Data, data.Host.ValueString())

	// create a new storage pool
	m := storagePoolResourceToModel(data)
	api := r.Data.K.ZoneAPI.CreateStoragePool(ctx, zoneId).StoragePool(m)
	if hostId != "" {
		api.HostId(hostId)
	}
	pool, _, err := api.Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	// set storage pool as default
	if data.Default.ValueBool() {
		_, err = r.Data.K.ZoneAPI.SetZoneDefaultStoragePool(ctx, zoneId, *pool.Id).Execute()
		if err != nil {
			errorCreateGeneric(resp, err)
			return
		}
	}

	data.ID = types.StringPointerValue(pool.Id)
	storagePoolModelToResource(pool, data) // read back resulting object
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

	pool, _, err := r.Data.K.PoolAPI.ReadStoragePool(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	storagePoolModelToResource(pool, data)
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

	m := storagePoolResourceToModel(data)
	_, _, err := r.Data.K.PoolAPI.UpdateStoragePool(ctx, data.ID.ValueString()).StoragePool(m).Execute()
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

	_, err := r.Data.K.PoolAPI.DeleteStoragePool(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
