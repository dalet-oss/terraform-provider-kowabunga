package kowabunga

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/vnet"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"
)

func resourceVNet() *schema.Resource {
	return &schema.Resource{
		Create: resourceVNetCreate,
		Read:   resourceVNetRead,
		Update: resourceVNetUpdate,
		Delete: resourceVNetDelete,
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
			KeySubnetID: {
				Type:     schema.TypeInt,
				Required: true,
			},
			KeyInterface: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyCIDR: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsCIDR,
			},
			KeyGateway: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsIPv4Address,
			},
			KeyDNS: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsIPv4Address,
			},
		},
	}
}

func newVNet(d *schema.ResourceData) models.VNet {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	subnetId := int64(d.Get(KeySubnetID).(int))
	itf := d.Get(KeyInterface).(string)
	cidr := d.Get(KeyCIDR).(string)
	gw := d.Get(KeyGateway).(string)
	dns := d.Get(KeyDNS).(string)

	return models.VNet{
		Name:        &name,
		Description: desc,
		SubnetID:    &subnetId,
		Interface:   &itf,
		Cidr:        &cidr,
		Gateway:     &gw,
		DNS:         &dns,
	}
}

func vnetToResource(v *models.VNet, d *schema.ResourceData) error {
	// set object params
	err := d.Set(KeyName, *v.Name)
	if err != nil {
		return err
	}
	err = d.Set(KeyDesc, v.Description)
	if err != nil {
		return err
	}
	err = d.Set(KeySubnetID, *v.SubnetID)
	if err != nil {
		return err
	}
	err = d.Set(KeyInterface, *v.Interface)
	if err != nil {
		return err
	}
	err = d.Set(KeyCIDR, *v.Cidr)
	if err != nil {
		return err
	}
	err = d.Set(KeyGateway, v.Gateway)
	if err != nil {
		return err
	}
	err = d.Set(KeyDNS, v.DNS)
	if err != nil {
		return err
	}
	return nil
}

func resourceVNetCreate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// find parent zone
	zoneId, err := zoneIDFromID(d, pconf)
	if err != nil {
		return err
	}

	// create a new virtual network
	cfg := newVNet(d)
	params := zone.NewCreateVNetParams().WithZoneID(zoneId).WithBody(&cfg)
	v, err := pconf.K.Zone.CreateVNet(params, nil)
	if err != nil {
		return err
	}

	// set resource ID accordingly
	d.SetId(v.Payload.ID)

	return nil
}

func resourceVNetRead(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := vnet.NewGetVNetParams().WithVnetID(d.Id())
	v, err := pconf.K.Vnet.GetVNet(params, nil)
	if err != nil {
		return err
	}

	// set object params
	return vnetToResource(v.Payload, d)
}

func resourceVNetDelete(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := vnet.NewDeleteVNetParams().WithVnetID(d.Id())
	_, err := pconf.K.Vnet.DeleteVNet(params, nil)
	if err != nil {
		return err
	}

	return nil
}

func resourceVNetUpdate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing region
	cfg := newVNet(d)
	params := vnet.NewUpdateVNetParams().WithVnetID(d.Id()).WithBody(&cfg)
	_, err := pconf.K.Vnet.UpdateVNet(params, nil)
	if err != nil {
		return err
	}

	return nil
}
