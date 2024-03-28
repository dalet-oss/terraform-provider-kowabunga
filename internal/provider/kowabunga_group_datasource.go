package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	GroupDataSourceName = "group"
)

var _ datasource.DataSource = &GroupDataSource{}
var _ datasource.DataSourceWithConfigure = &GroupDataSource{}

func NewGroupDataSource() datasource.DataSource {
	return &GroupDataSource{}
}

type GroupDataSource struct {
	Data *KowabungaProviderData
}

func (d *GroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	datasourceMetadata(req, resp, GroupDataSourceName)
}

func (d *GroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.Data = datasourceConfigure(req, resp)
}

func (d *GroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	datasourceFilteredSchema(resp, GroupDataSourceName)
}

func (d *GroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GenericDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d.Data.Mutex.Lock()
	defer d.Data.Mutex.Unlock()

	groups, _, err := d.Data.K.GroupAPI.ListGroups(ctx).Execute()
	if err != nil {
		errorDataSourceReadGeneric(resp, err)
		return
	}
	for _, rg := range groups {
		r, _, err := d.Data.K.GroupAPI.ReadGroup(ctx, rg).Execute()
		if err == nil && r.Name == data.Name.ValueString() {
			data.ID = types.StringPointerValue(r.Id)
			break
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
