package provider

import (
	"context"
	"strings"

	"golang.org/x/exp/maps"

	sdk "github.com/dalet-oss/kowabunga-api/sdk/go/client"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	UserResourceName = "user"

	UserDefaultValueNotifications = false
)

var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

var userSupportedRoles = []string{
	"superAdmin",
	"projectAdmin",
	"user",
}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserResource struct {
	Data *KowabungaProviderData
}

type UserResourceModel struct {
	ID            types.String   `tfsdk:"id"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
	Name          types.String   `tfsdk:"name"`
	Email         types.String   `tfsdk:"email"`
	Password      types.String   `tfsdk:"password"`
	Role          types.String   `tfsdk:"role"`
	Notifications types.Bool     `tfsdk:"notifications"`
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resourceMetadata(req, resp, UserResourceName)
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceImportState(ctx, req, resp)
	resource.ImportStatePassthroughID(ctx, path.Root(KeyPassword), req, resp)
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Data = resourceConfigure(req, resp)
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Kowabunga user resource",
		Attributes: map[string]schema.Attribute{
			KeyEmail: schema.StringAttribute{
				MarkdownDescription: "Kowabunga user email address",
				Required:            true,
				Validators: []validator.String{
					&stringUserEmailValidator{},
				},
			},
			KeyPassword: schema.StringAttribute{
				MarkdownDescription: "Kowabunga user password",
				Required:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			KeyRole: schema.StringAttribute{
				MarkdownDescription: "Kowabunga user role (" + strings.Join(userSupportedRoles, ", ") + ")",
				Required:            true,
				Validators: []validator.String{
					&stringUserRoleValidator{},
				},
			},
			KeyNotifications: schema.BoolAttribute{
				MarkdownDescription: "Whether Kowabunga user wants email notifications en events (default: **false**)",
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(UserDefaultValueNotifications),
			},
		},
	}
	maps.Copy(resp.Schema.Attributes, resourceAttributes(&ctx))
}

// converts user from Terraform model to Kowabunga API model
func userResourceToModel(d *UserResourceModel) sdk.User {
	return sdk.User{
		Name:          d.Name.ValueString(),
		Email:         d.Email.ValueString(),
		Password:      d.Password.ValueString(),
		Role:          d.Role.ValueString(),
		Notifications: d.Notifications.ValueBoolPointer(),
	}
}

// converts user from Kowabunga API model to Terraform model
func userModelToResource(r *sdk.User, d *UserResourceModel) {
	if r == nil {
		return
	}

	d.Name = types.StringValue(r.Name)
	d.Email = types.StringValue(r.Email)
	d.Password = types.StringValue(r.Password)
	d.Role = types.StringValue(r.Role)
	if r.Notifications != nil {
		d.Notifications = types.BoolPointerValue(r.Notifications)
	} else {
		d.Notifications = types.BoolValue(UserDefaultValueNotifications)
	}
}

// converts token from Kowabunga API model to Terraform model
// func tokenModelToUserResource(r *sdk.ApiToken, d *UserResourceModel) {
// 	if r == nil {
// 		return
// 	}

// 	d.ApiKey = types.StringPointerValue(r.ApiKey)
// }

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *UserResourceModel
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

	m := userResourceToModel(data)
	user, _, err := r.Data.K.UserAPI.CreateUser(ctx).User(m).Execute()
	if err != nil {
		errorCreateGeneric(resp, err)
		return
	}
	data.ID = types.StringPointerValue(user.Id)
	userModelToResource(user, data) // read back resulting object

	// create a new authentication token
	// token, _, err := r.Data.K.UserAPI.SetUserApiToken(ctx, *user.Id).Expire(false).Execute()
	// if err != nil {
	// 	errorCreateGeneric(resp, err)
	// 	return
	// }
	// tokenModelToUserResource(token, data) // read back resulting object

	tflog.Trace(ctx, "created user resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *UserResourceModel
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

	user, _, err := r.Data.K.UserAPI.ReadUser(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorReadGeneric(resp, err)
		return
	}

	userModelToResource(user, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *UserResourceModel
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

	m := userResourceToModel(data)
	_, _, err := r.Data.K.UserAPI.UpdateUser(ctx, data.ID.ValueString()).User(m).Execute()
	if err != nil {
		errorUpdateGeneric(resp, err)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *UserResourceModel
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

	_, err := r.Data.K.UserAPI.DeleteUser(ctx, data.ID.ValueString()).Execute()
	if err != nil {
		errorDeleteGeneric(resp, err)
		return
	}
	tflog.Trace(ctx, "Deleted "+data.ID.ValueString())
}
