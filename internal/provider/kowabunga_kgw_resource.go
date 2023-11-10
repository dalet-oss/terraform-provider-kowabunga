package provider

import (
	"context"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/kgw"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/project"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/exp/maps"
)

const (
	KgwResourceName = "kgw"
)

var _ resource.Resource = &KgwResource{}
var _ resource.ResourceWithImportState = &KgwResource{}

func NewKgwResource() resource.Resource {
	return &KgwResource{}
}

type KgwResource struct {
	Data *KowabungaProviderData
}

type KgwResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Desc      types.String `tfsdk:"desc"`
	Project   types.String `tfsdk:"project"`
	Zone      types.String `tfsdk:"zone"`
	Pool      types.String `tfsdk:"pool"`
	PublicIp  types.String `tfsdk:"publicips"`
	PrivateIp types.String `tfsdk:"private"`
	Nats      types.List   `tfsdk:"nats"`
}

type KgwNat struct {
	private_ip string
	public_ip  string
	ports      []uint16
}

func (r *KgwResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, KgwResourceName)
}

func (r *KgwResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyPrivateIp), req, resp)
}

func (r *KgwResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *KgwResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a KGW resource. **KGW** (stands for *Kowabunga Gateway*) is a resource that provides Nats & internet access capabilities for a given project.",
		Attributes: map[string]schema.Attribute{
			KeyProject: schema.StringAttribute{
				MarkdownDescription: "Associated project name or ID",
				Required:            true,
			},
			KeyZone: schema.StringAttribute{
				MarkdownDescription: "Associated zone name or ID",
				Required:            true,
			},
			KeyPublicIp: schema.StringAttribute{
				MarkdownDescription: "The KGW default Public IP (read-only)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyPrivateIp: schema.StringAttribute{
				MarkdownDescription: "The KGW Private IP (read-only)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyNats: schema.ListAttribute{
				MarkdownDescription: "NATs Configuration",
				ElementType:         basetypes.MapType{},
				Optional:            true,
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts kgw from Terraform model to Kowabunga API model
func kgwResourceToModel(d *KgwResourceModel) models.KGW {
	nats := []KgwNat{}
	d.Nats.ElementsAs(context.TODO(), &nats, false)
	natsModel := []*models.KGWNat{}

	for _, v := range nats {
		nat := models.KGWNat{
			PrivateIP: v.private_ip,
			PublicIP:  v.public_ip,
			Ports:     v.ports,
		}
		natsModel = append(natsModel, &nat)
	}
	return models.KGW{
		Description: d.Desc.ValueString(),
		PublicIP:    d.PublicIp.ValueString(),
		PrivateIP:   d.PrivateIp.ValueString(),
		Nats:        natsModel,
	}
}

// converts kgw from Kowabunga API model to Terraform model
func kgwModelToResource(r *models.KGW, d *KgwResourceModel) {
	if r == nil {
		return
	}
	d.Desc = types.StringValue(r.Description)
	d.PublicIp = types.StringValue(r.PublicIP)
	d.PrivateIp = types.StringValue(r.PrivateIP)

	nats := []attr.Value{}
	natType := map[string]attr.Type{
		"private_ip": types.StringType,
		"public_ip":  types.StringType,
		"port":       types.ListType{},
	}
	for _, nat := range r.Nats {
		ports := []attr.Value{}
		for _, port := range nat.Ports {
			ports = append(ports, types.Int64Value(int64(port)))
		}
		portValues, _ := types.ListValue(types.Int64Type, ports)
		a := map[string]attr.Value{
			"private_ip": types.StringValue(nat.PrivateIP),
			"public_ip":  types.StringValue(nat.PublicIP),
			"ports":      portValues,
		}
		object, _ := types.ObjectValue(natType, a)
		nats = append(nats, object)
	}
	d.Nats, _ = types.ListValue(types.ObjectType{}, nats)
}

func (r *KgwResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *KgwResourceModel
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

	// create a new KGW
	cfg := kgwResourceToModel(data)
	params := project.NewCreateProjectZoneKgwParams().
		WithProjectID(projectId).WithZoneID(zoneId).
		WithBody(&cfg)

	obj, err := r.Data.K.Project.CreateProjectZoneKgw(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	kgwModelToResource(obj.Payload, data) // read back resulting object
	tflog.Trace(ctx, "created KGW resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KgwResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *KgwResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := kgw.NewGetKgwParams().WithKgwID(data.ID.ValueString())
	obj, err := r.Data.K.Kgw.GetKgw(params, nil)
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	kgwModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KgwResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *KgwResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := kgwResourceToModel(data)
	params := kgw.NewUpdateKGWParams().WithKgwID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Kgw.UpdateKGW(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KgwResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *KgwResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := kgw.NewDeleteKGWParams().WithKgwID(data.ID.ValueString())
	_, err := r.Data.K.Kgw.DeleteKGW(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
