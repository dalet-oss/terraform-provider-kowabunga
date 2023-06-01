package kowabunga

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/netgw"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"
)

func resourceNetGW() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetGWCreate,
		Read:   resourceNetGWRead,
		Update: resourceNetGWUpdate,
		Delete: resourceNetGWDelete,
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

func gwToResource(gw *models.NetGW, d *schema.ResourceData) error {
	// set object params
	err := d.Set(KeyName, *gw.Name)
	if err != nil {
		return err
	}
	err = d.Set(KeyDesc, gw.Description)
	if err != nil {
		return err
	}
	err = d.Set(KeyAddress, *gw.Address)
	if err != nil {
		return err
	}
	err = d.Set(KeyPort, *gw.Port)
	if err != nil {
		return err
	}
	err = d.Set(KeyToken, *gw.Token)
	if err != nil {
		return err
	}
	return nil
}

func resourceNetGWCreate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// find parent zone
	zoneId, err := zoneIDFromID(d, pconf)
	if err != nil {
		return err
	}

	// create a new network gateway
	cfg := newNetGW(d)
	params := zone.NewCreateNetGWParams().WithZoneID(zoneId).WithBody(&cfg)
	gw, err := pconf.K.Zone.CreateNetGW(params, nil)
	if err != nil {
		return err
	}

	// set resource ID accordingly
	d.SetId(gw.Payload.ID)

	return nil
}

func resourceNetGWRead(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := netgw.NewGetNetGWParams().WithNetgwID(d.Id())
	gw, err := pconf.K.Netgw.GetNetGW(params, nil)
	if err != nil {
		return err
	}

	// set object params
	return gwToResource(gw.Payload, d)
}

func resourceNetGWDelete(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := netgw.NewDeleteNetGWParams().WithNetgwID(d.Id())
	_, err := pconf.K.Netgw.DeleteNetGW(params, nil)
	if err != nil {
		return err
	}

	return nil
}

func resourceNetGWUpdate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing network gateway
	cfg := newNetGW(d)
	params := netgw.NewUpdateNetGWParams().WithNetgwID(d.Id()).WithBody(&cfg)
	_, err := pconf.K.Netgw.UpdateNetGW(params, nil)
	if err != nil {
		return err
	}

	return nil
}
