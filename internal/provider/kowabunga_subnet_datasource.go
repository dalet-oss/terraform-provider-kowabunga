package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	SubnetDataSourceName = "subnet"
)

var _ datasource.DataSource = &SubnetDataSource{}
var _ datasource.DataSourceWithConfigure = &SubnetDataSource{}

func NewSubnetDataSource() datasource.DataSource {
	return &SubnetDataSource{}
}

type SubnetDataSource struct {
	Data *KowabungaProviderData
}

func (d *SubnetDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	datasourceMetadata(req, resp, SubnetDataSourceName)
}

func (d *SubnetDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.Data = datasourceConfigure(req, resp)
}

func (d *SubnetDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	datasourceFilteredSchema(resp, SubnetDataSourceName)
}

func (d *SubnetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GenericDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d.Data.Mutex.Lock()
	defer d.Data.Mutex.Unlock()

	subnets, _, err := d.Data.K.SubnetAPI.ListSubnets(ctx).Execute()
	if err != nil {
		errorDataSourceReadGeneric(resp, err)
		return
	}
	for _, rg := range subnets {
		r, _, err := d.Data.K.SubnetAPI.ReadSubnet(ctx, rg).Execute()
		if err == nil && r.Name == data.Name.ValueString() {
			data.ID = types.StringPointerValue(r.Id)
			break
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
