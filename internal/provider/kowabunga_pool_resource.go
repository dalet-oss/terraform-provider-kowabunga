package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/client/pool"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	PoolResourceName = "pool"
)

var _ resource.Resource = &PoolResource{}
var _ resource.ResourceWithImportState = &PoolResource{}

func NewPoolResource() resource.Resource {
	return &PoolResource{}
}

type PoolResource struct {
	Data *KowabungaProviderData
}

type PoolResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Desc     types.String `tfsdk:"desc"`
	Zone     types.String `tfsdk:"zone"`
	Pool     types.String `tfsdk:"pool"`
	Address  types.String `tfsdk:"address"`
	Port     types.Int64  `tfsdk:"port"`
	Secret   types.String `tfsdk:"secret"`
	Price    types.Int64  `tfsdk:"price"`
	Currency types.String `tfsdk:"currency"`
	Default  types.Bool   `tfsdk:"default"`
}

func (r *PoolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, PoolResourceName)
}

func (r *PoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *PoolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *PoolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a pool resource",
		Attributes: map[string]schema.Attribute{
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
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
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
					int64validator.AtMost(65535),
				},
			},
			KeySecret: schema.StringAttribute{
				MarkdownDescription: "CephX client authentication UUID",
				Optional:            true,
				Sensitive:           true,
			},
			KeyPrice: schema.Int64Attribute{
				MarkdownDescription: "libvirt host monthly price value",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyCurrency: schema.StringAttribute{
				MarkdownDescription: "libvirt host monthly price currency",
				Computed:            true,
				Optional:            true,
				Default:             stringdefault.StaticString("EUR"),
			},
			KeyDefault: schema.BoolAttribute{
				MarkdownDescription: "Whether to set pool as zone's default one",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts pool from Terraform model to Kowabunga API model
func poolResourceToModel(d *PoolResourceModel) models.StoragePool {
	cost := models.Cost{
		Price:    d.Price.ValueInt64Pointer(),
		Currency: d.Currency.ValueStringPointer(),
	}
	return models.StoragePool{
		Name:           d.Name.ValueStringPointer(),
		Description:    d.Desc.ValueString(),
		Pool:           d.Pool.ValueStringPointer(),
		CephAddress:    d.Address.ValueStringPointer(),
		CephPort:       d.Port.ValueInt64Pointer(),
		CephSecretUUID: d.Secret.ValueString(),
		Cost:           &cost,
	}
}

// converts pool from Kowabunga API model to Terraform model
func poolModelToResource(r *models.StoragePool, d *PoolResourceModel) {
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	d.Pool = types.StringPointerValue(r.Pool)
	d.Address = types.StringPointerValue(r.CephAddress)
	d.Port = types.Int64PointerValue(r.CephPort)
	d.Secret = types.StringValue(r.CephSecretUUID)
	if r.Cost != nil {
		d.Price = types.Int64PointerValue(r.Cost.Price)
		d.Currency = types.StringPointerValue(r.Cost.Currency)
	}
}

func (r *PoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *PoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	// find parent zone
	zoneId, err := getZoneID(r.Data, data.Zone.ValueString())
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	// create a new network gateway
	cfg := poolResourceToModel(data)
	params := zone.NewCreatePoolParams().WithZoneID(zoneId).WithBody(&cfg)
	obj, err := r.Data.K.Zone.CreatePool(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	// set pool as default
	if data.Default.ValueBool() {
		params2 := zone.NewUpdateZoneDefaultPoolParams().WithZoneID(zoneId).WithPoolID(obj.Payload.ID)
		_, err = r.Data.K.Zone.UpdateZoneDefaultPool(params2, nil)
		if err != nil {
			errorCreateGeneric(resp, err)
			return
		}
	}

	data.ID = types.StringValue(obj.Payload.ID)
	tflog.Trace(ctx, "created pool resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *PoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := pool.NewGetPoolParams().WithPoolID(data.ID.ValueString())
	obj, err := r.Data.K.Pool.GetPool(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	poolModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *PoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := poolResourceToModel(data)
	params := pool.NewUpdatePoolParams().WithPoolID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Pool.UpdatePool(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *PoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := pool.NewDeletePoolParams().WithPoolID(data.ID.ValueString())
	_, err := r.Data.K.Pool.DeletePool(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
