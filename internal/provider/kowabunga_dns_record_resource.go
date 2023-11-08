package provider

import (
	"context"
	"golang.org/x/exp/maps"

	"github.com/dalet-oss/kowabunga-api/sdk/go/client/project"
	"github.com/dalet-oss/kowabunga-api/sdk/go/client/record"
	"github.com/dalet-oss/kowabunga-api/sdk/go/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	DnsRecordResourceName = "dns_record"
)

var _ resource.Resource = &DnsRecordResource{}
var _ resource.ResourceWithImportState = &DnsRecordResource{}

func NewDnsRecordResource() resource.Resource {
	return &DnsRecordResource{}
}

type DnsRecordResource struct {
	Data *KowabungaProviderData
}

type DnsRecordResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Desc      types.String `tfsdk:"desc"`
	Project   types.String `tfsdk:"project"`
	Addresses types.List   `tfsdk:"addresses"`
}

func (r *DnsRecordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, DnsRecordResourceName)
}

func (r *DnsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
}

func (r *DnsRecordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *DnsRecordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DNS record resource",
		Attributes: map[string]schema.Attribute{
			KeyProject: schema.StringAttribute{
				MarkdownDescription: "Associated project name or ID",
				Required:            true,
			},
			KeyAddresses: schema.ListAttribute{
				MarkdownDescription: "The list of IPv4 addresses to be associated with the DNS record",
				ElementType:         types.StringType,
				Required:            true,
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes())
}

// converts record from Terraform model to Kowabunga API model
func recordResourceToModel(d *DnsRecordResourceModel) models.DNSRecord {
	addresses := []string{}
	d.Addresses.ElementsAs(context.TODO(), &addresses, false)
	return models.DNSRecord{
		Name:        d.Name.ValueStringPointer(),
		Description: d.Desc.ValueString(),
		Addresses:   addresses,
	}
}

// converts record from Kowabunga API model to Terraform model
func recordModelToResource(r *models.DNSRecord, d *DnsRecordResourceModel) {
	d.Name = types.StringPointerValue(r.Name)
	d.Desc = types.StringValue(r.Description)
	addresses := []attr.Value{}
	for _, a := range r.Addresses {
		addresses = append(addresses, types.StringValue(a))
	}
	d.Addresses, _ = types.ListValue(types.StringType, addresses)
}

func (r *DnsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *DnsRecordResourceModel
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

	// create a new record
	cfg := recordResourceToModel(data)
	params := project.NewCreateProjectDNSRecordParams().WithProjectID(projectId).WithBody(&cfg)
	obj, err := r.Data.K.Project.CreateProjectDNSRecord(params, nil)
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}

	data.ID = types.StringValue(obj.Payload.ID)
	tflog.Trace(ctx, "created DNS record resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *DnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := record.NewGetDNSRecordParams().WithRecordID(data.ID.ValueString())
	obj, err := r.Data.K.Record.GetDNSRecord(params, nil)
	if err != nil {
		tflog.Trace(ctx, err.Error())
		errorReadGeneric(resp, err)
		return
	}

	recordModelToResource(obj.Payload, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *DnsRecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	cfg := recordResourceToModel(data)
	params := record.NewUpdateDNSRecordParams().WithRecordID(data.ID.ValueString()).WithBody(&cfg)
	_, err := r.Data.K.Record.UpdateDNSRecord(params, nil)
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DnsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *DnsRecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.Data.Mutex.Lock()
	defer r.Data.Mutex.Unlock()

	params := record.NewDeleteDNSRecordParams().WithRecordID(data.ID.ValueString())
	_, err := r.Data.K.Record.DeleteDNSRecord(params, nil)
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
}
