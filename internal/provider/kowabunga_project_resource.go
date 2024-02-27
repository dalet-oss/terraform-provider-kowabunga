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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	ProjectResourceName = "project"
)

var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

type ProjectResource struct {
	Data *KowabungaProviderData
}

type ProjectResourceModel struct {
	ID             types.String   `tfsdk:"id"`
	Timeouts       timeouts.Value `tfsdk:"timeouts"`
	Name           types.String   `tfsdk:"name"`
	Desc           types.String   `tfsdk:"desc"`
	Owner          types.String   `tfsdk:"owner"`
	Email          types.String   `tfsdk:"email"`
	Domain         types.String   `tfsdk:"domain"`
	SubnetSize     types.Int64    `tfsdk:"subnet_size"`
	RootPassword   types.String   `tfsdk:"root_password"`
	User           types.String   `tfsdk:"bootstrap_user"`
	Pubkey         types.String   `tfsdk:"bootstrap_pubkey"`
	Tags           types.List     `tfsdk:"tags"`
	Metadatas      types.Map      `tfsdk:"metadata"`
	MaxInstances   types.Int64    `tfsdk:"max_instances"`
	MaxMemory      types.Int64    `tfsdk:"max_memory"`
	MaxStorage     types.Int64    `tfsdk:"max_storage"`
	MaxVCPUs       types.Int64    `tfsdk:"max_vcpus"`
	Notify         types.Bool     `tfsdk:"notify"`
	PrivateSubnets types.Map      `tfsdk:"private_subnets"`
}

type ProjectQuotaModel struct {
}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, ProjectResourceName)
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyPrivateSubnets), req, resp)
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a project resource",
		Attributes: map[string]schema.Attribute{
			KeyOwner: schema.StringAttribute{
				MarkdownDescription: "Owner of the project.",
				Required:            true,
			},
			KeyEmail: schema.StringAttribute{
				MarkdownDescription: "Email associated to the project to receive notifications.",
				Required:            true,
			},
			KeyDomain: schema.StringAttribute{
				MarkdownDescription: "Internal domain name associated to the project (e.g. myproject.acme.com). (default: none)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			KeySubnetSize: schema.Int64Attribute{
				MarkdownDescription: "Project requested VPC subnet size (defaults to /26)",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(26),
			},
			KeyRootPassword: schema.StringAttribute{
				MarkdownDescription: "The project default root password, set at cloud-init instance bootstrap phase. Will be randomly auto-generated at each instance creation if unspecified.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			KeyBootstrapUser: schema.StringAttribute{
				MarkdownDescription: "The project default service user name, created at cloud-init instance bootstrap phase. Will use Kowabunga's default configuration one if unspecified.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyBootstrapPubkey: schema.StringAttribute{
				MarkdownDescription: "The project default public SSH key, to be associated to bootstrap user. Will use Kowabunga's default configuration one if unspecified.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyTags: schema.ListAttribute{
				MarkdownDescription: "List of tags associated with the project",
				ElementType:         types.StringType,
				Required:            true,
			},
			KeyMetadata: schema.MapAttribute{
				MarkdownDescription: "List of metadatas key/value associated with the project",
				ElementType:         types.StringType,
				Required:            true,
			},
			KeyMaxInstances: schema.Int64Attribute{
				MarkdownDescription: "Project maximum deployable instances. Defaults to 0 (unlimited).",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyMaxMemory: schema.Int64Attribute{
				MarkdownDescription: "Project maximum usable memory (expressed in GB). Defaults to 0 (unlimited).",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyMaxStorage: schema.Int64Attribute{
				MarkdownDescription: "Project maximum usable storage (expressed in GB). Defaults to 0 (unlimited).",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyMaxVCPUs: schema.Int64Attribute{
				MarkdownDescription: "Project maximum usable virtual CPUs. Defaults to 0 (unlimited).",
				Computed:            true,
				Optional:            true,
				Default:             int64default.StaticInt64(0),
			},
			KeyNotify: schema.BoolAttribute{
				MarkdownDescription: "Whether to send email notification at creation (default: **true**)",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
			},
			KeyPrivateSubnets: schema.MapAttribute{
				Computed:            true,
				MarkdownDescription: "List of project's private subnets zones association (read-only)",
				ElementType:         types.StringType,
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts project from Terraform model to Kowabunga API model
func projectResourceToModel(d *ProjectResourceModel) sdk.Project {
	tags := []string{}
	d.Tags.ElementsAs(context.TODO(), &tags, false)
	metas := map[string]string{}
	d.Metadatas.ElementsAs(context.TODO(), &metas, false)
	metadatas := []sdk.Metadata{}
	for k, v := range metas {
		m := sdk.Metadata{
			Key:   &k,
			Value: &v,
		}
		metadatas = append(metadatas, m)
	}

	instances := int32(d.MaxInstances.ValueInt64())
	memory := int64(d.MaxMemory.ValueInt64()) * HelperGbToBytes
	storage := int64(d.MaxStorage.ValueInt64()) * HelperGbToBytes
	vcpus := int32(d.MaxVCPUs.ValueInt64())
	quotas := sdk.ProjectResources{
		Instances: &instances,
		Memory:    &memory,
		Storage:   &storage,
		Vcpus:     &vcpus,
	}

	return sdk.Project{
		Name:            d.Name.ValueString(),
		Description:     d.Desc.ValueStringPointer(),
		Owner:           d.Owner.ValueString(),
		Email:           d.Email.ValueString(),
		Domain:          d.Domain.ValueStringPointer(),
		RootPassword:    d.RootPassword.ValueStringPointer(),
		BootstrapUser:   d.User.ValueStringPointer(),
		BootstrapPubkey: d.Pubkey.ValueStringPointer(),
		Tags:            tags,
		Metadatas:       metadatas,
		Quotas:          quotas,
	}
}

// converts project from Kowabunga API model to Terraform model
func projectModelToResource(r *sdk.Project, d *ProjectResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringValue(r.Name)
	d.Desc = types.StringPointerValue(r.Description)
	d.Owner = types.StringValue(r.Owner)
	d.Email = types.StringValue(r.Email)
	d.Domain = types.StringPointerValue(r.Domain)
	d.RootPassword = types.StringPointerValue(r.RootPassword)
	d.User = types.StringPointerValue(r.BootstrapUser)
	d.Pubkey = types.StringPointerValue(r.BootstrapPubkey)
	tags := []attr.Value{}
	for _, t := range r.Tags {
		tags = append(tags, types.StringValue(t))
	}
	d.Tags, _ = types.ListValue(types.StringType, tags)
	metadatas := map[string]attr.Value{}
	for _, m := range r.Metadatas {
		metadatas[*m.Key] = types.StringPointerValue(m.Value)
	}
	d.Metadatas = basetypes.NewMapValueMust(types.StringType, metadatas)
	d.MaxInstances = types.Int64Value(int64(*r.Quotas.Instances))
	d.MaxMemory = types.Int64Value(int64(*r.Quotas.Memory) / HelperGbToBytes)
	d.MaxStorage = types.Int64Value(int64(*r.Quotas.Storage) / HelperGbToBytes)
	d.MaxVCPUs = types.Int64Value(int64(*r.Quotas.Vcpus))

	privateSubnets := map[string]attr.Value{}
	for _, p := range r.PrivateSubnets {
		privateSubnets[*p.Key] = types.StringPointerValue(p.Value)
	}
	d.PrivateSubnets = basetypes.NewMapValueMust(types.StringType, privateSubnets)
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ProjectResourceModel
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

	// create a new project
	m := projectResourceToModel(data)
	project, _, err := r.Data.K.ProjectAPI.CreateProject(ctx).Project(m).SubnetSize(int32(data.SubnetSize.ValueInt64())).Notify(data.Notify.ValueBool()).Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringPointerValue(project.Id)
	projectModelToResource(project, data) // read back resulting object

	tflog.Trace(ctx, "created project resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *ProjectResourceModel
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

	project, _, err := r.Data.K.ProjectAPI.ReadProject(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	projectModelToResource(project, data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ProjectResourceModel
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

	m := projectResourceToModel(data)
	_, _, err := r.Data.K.ProjectAPI.UpdateProject(ctx, data.ID.ValueString()).Project(m).Execute()
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *ProjectResourceModel
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

	_, err := r.Data.K.ProjectAPI.DeleteProject(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
