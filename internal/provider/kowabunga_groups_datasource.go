package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	GroupsDataSourceName = "groups"
)

var _ datasource.DataSource = &GroupsDataSource{}
var _ datasource.DataSourceWithConfigure = &GroupsDataSource{}

func NewGroupsDataSource() datasource.DataSource {
	return &GroupsDataSource{}
}

type GroupsDataSource struct {
	Data *KowabungaProviderData
}

type GroupsDataSourceModel struct {
	Groups map[string]types.String `tfsdk:"groups"`
}

func (d *GroupsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	datasourceMetadata(req, resp, GroupsDataSourceName)
}

func (d *GroupsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.Data = datasourceConfigure(req, resp)
}

func (d *GroupsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	datasourceFullSchema(resp, GroupsDataSourceName)
}

func (d *GroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GroupsDataSourceModel
	d.Data.Mutex.Lock()
	defer d.Data.Mutex.Unlock()

	groups, _, err := d.Data.K.GroupAPI.ListGroups(ctx).Execute()
	if err != nil {
		errorDataSourceReadGeneric(resp, err)
		return
	}
	data.Groups = map[string]types.String{}
	for _, rg := range groups {
		r, _, err := d.Data.K.GroupAPI.ReadGroup(ctx, rg).Execute()
		if err != nil {
			continue
		}
		data.Groups[r.Name] = types.StringPointerValue(r.Id)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
