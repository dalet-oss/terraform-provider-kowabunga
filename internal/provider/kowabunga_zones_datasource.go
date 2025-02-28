/*
 * Copyright (c) The Kowabunga Project
 * Apache License, Version 2.0 (see LICENSE or https://www.apache.org/licenses/LICENSE-2.0.txt)
 * SPDX-License-Identifier: Apache-2.0
 */

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	ZonesDataSourceName = "zones"
)

var _ datasource.DataSource = &ZonesDataSource{}
var _ datasource.DataSourceWithConfigure = &ZonesDataSource{}

func NewZonesDataSource() datasource.DataSource {
	return &ZonesDataSource{}
}

type ZonesDataSource struct {
	Data *KowabungaProviderData
}

type ZonesDataSourceModel struct {
	Zones map[string]types.String `tfsdk:"zones"`
}

func (d *ZonesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	datasourceMetadata(req, resp, ZonesDataSourceName)
}

func (d *ZonesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.Data = datasourceConfigure(req, resp)
}

func (d *ZonesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	datasourceFullSchema(resp, ZonesDataSourceName)
}

func (d *ZonesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ZonesDataSourceModel
	d.Data.Mutex.Lock()
	defer d.Data.Mutex.Unlock()

	zones, _, err := d.Data.K.ZoneAPI.ListZones(ctx).Execute()
	if err != nil {
		errorDataSourceReadGeneric(resp, err)
		return
	}
	data.Zones = map[string]types.String{}
	for _, rg := range zones {
		r, _, err := d.Data.K.ZoneAPI.ReadZone(ctx, rg).Execute()
		if err != nil {
			continue
		}
		data.Zones[r.Name] = types.StringPointerValue(r.Id)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
