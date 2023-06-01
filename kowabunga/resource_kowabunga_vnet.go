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
			KeyVLAN: {
				Type:     schema.TypeInt,
				Required: true,
			},
			KeyInterface: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyPrivate: {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			KeyDefault: {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			KeySubnet: {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
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
				},
			},
		},
	}
}

func newVNet(d *schema.ResourceData) models.VNet {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	vlan := int64(d.Get(KeyVLAN).(int))
	itf := d.Get(KeyInterface).(string)
	private := d.Get(KeyPrivate).(bool)

	vnet := models.VNet{
		Name:        &name,
		Description: desc,
		Vlan:        &vlan,
		Interface:   &itf,
		Private:     &private,
	}

	for _, s := range d.Get(KeySubnet).([]interface{}) {
		sub := s.(map[string]interface{})
		cidr := sub[KeyCIDR].(string)
		gw := sub[KeyGateway].(string)
		dns := sub[KeyDNS].(string)
		subnet := models.Subnet{
			Cidr:    &cidr,
			Gateway: &gw,
			DNS:     dns,
		}
		vnet.Subnets = append(vnet.Subnets, &subnet)
	}

	return vnet
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
	err = d.Set(KeyVLAN, *v.Vlan)
	if err != nil {
		return err
	}
	err = d.Set(KeyInterface, *v.Interface)
	if err != nil {
		return err
	}
	err = d.Set(KeyPrivate, *v.Private)
	if err != nil {
		return err
	}

	var subnets []map[string]interface{}
	for _, s := range v.Subnets {
		sub := map[string]interface{}{
			KeyCIDR:    *s.Cidr,
			KeyGateway: *s.Gateway,
			KeyDNS:     s.DNS,
		}
		subnets = append(subnets, sub)
	}
	if len(subnets) > 0 {
		err := d.Set(KeySubnet, subnets)
		if err != nil {
			return err
		}
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

	// set virtual network as default
	dflt := d.Get(KeyPrivate).(bool)
	if dflt {
		params2 := zone.NewUpdateZoneDefaultVNetParams().WithZoneID(zoneId).WithVnetID(v.Payload.ID)
		_, err = pconf.K.Zone.UpdateZoneDefaultVNet(params2, nil)
		if err != nil {
			return err
		}
	}

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
