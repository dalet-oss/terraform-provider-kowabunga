package kowabunga

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/netgw"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"
)

func resourceNetGW() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceNetGWCreate,
		ReadContext:   resourceNetGWRead,
		UpdateContext: resourceNetGWUpdate,
		DeleteContext: resourceNetGWDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			KeyZone: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyName: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyDesc: {
				Type:     schema.TypeString,
				Optional: true,
			},
			KeyAddress: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsIPv4Address,
			},
			KeyPort: {
				Type:     schema.TypeInt,
				Optional: true,
			},
			KeyToken: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
		},
	}
}

func newNetGW(d *schema.ResourceData) models.NetGW {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	address := d.Get(KeyAddress).(string)
	port := int64(d.Get(KeyPort).(int))
	token := d.Get(KeyToken).(string)

	return models.NetGW{
		Name:        &name,
		Description: desc,
		Address:     &address,
		Port:        &port,
		Token:       &token,
	}
}

func gwToResource(gw *models.NetGW, d *schema.ResourceData) diag.Diagnostics {
	// set object params
	err := d.Set(KeyName, *gw.Name)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyDesc, gw.Description)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyAddress, *gw.Address)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyPort, *gw.Port)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyToken, *gw.Token)
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceNetGWCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// find parent zone
	zoneId, err := zoneIDFromID(d, pconf)
	if err != nil {
		return diag.FromErr(err)
	}

	// create a new network gateway
	cfg := newNetGW(d)
	params := zone.NewCreateNetGWParams().WithZoneID(zoneId).WithBody(&cfg)
	gw, err := pconf.K.Zone.CreateNetGW(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	// set resource ID accordingly
	d.SetId(gw.Payload.ID)

	return nil
}

func resourceNetGWRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := netgw.NewGetNetGWParams().WithNetgwID(d.Id())
	gw, err := pconf.K.Netgw.GetNetGW(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	// set object params
	return gwToResource(gw.Payload, d)
}

func resourceNetGWDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := netgw.NewDeleteNetGWParams().WithNetgwID(d.Id())
	_, err := pconf.K.Netgw.DeleteNetGW(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceNetGWUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing network gateway
	cfg := newNetGW(d)
	params := netgw.NewUpdateNetGWParams().WithNetgwID(d.Id()).WithBody(&cfg)
	_, err := pconf.K.Netgw.UpdateNetGW(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
