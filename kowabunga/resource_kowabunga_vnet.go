package kowabunga

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/vnet"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"
)

func resourceVNet() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceVNetCreate,
		ReadContext:   resourceVNetRead,
		UpdateContext: resourceVNetUpdate,
		DeleteContext: resourceVNetDelete,
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
		},
	}
}

func newVNet(d *schema.ResourceData) models.VNet {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	vlan := int64(d.Get(KeyVLAN).(int))
	itf := d.Get(KeyInterface).(string)
	private := d.Get(KeyPrivate).(bool)

	return models.VNet{
		Name:        &name,
		Description: desc,
		Vlan:        &vlan,
		Interface:   &itf,
		Private:     &private,
	}
}

func vnetToResource(v *models.VNet, d *schema.ResourceData) diag.Diagnostics {
	// set object params
	err := d.Set(KeyName, *v.Name)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyDesc, v.Description)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyVLAN, *v.Vlan)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyInterface, *v.Interface)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyPrivate, *v.Private)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceVNetCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// find parent zone
	zoneId, err := zoneIDFromID(d, pconf)
	if err != nil {
		return diag.FromErr(err)
	}

	// create a new virtual network
	cfg := newVNet(d)
	params := zone.NewCreateVNetParams().WithZoneID(zoneId).WithBody(&cfg)
	v, err := pconf.K.Zone.CreateVNet(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	// set resource ID accordingly
	d.SetId(v.Payload.ID)

	// set virtual network as default
	dflt := d.Get(KeyDefault).(bool)
	if dflt {
		params2 := zone.NewUpdateZoneDefaultVNetParams().WithZoneID(zoneId).WithVnetID(v.Payload.ID)
		_, err = pconf.K.Zone.UpdateZoneDefaultVNet(params2, nil)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceVNetRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := vnet.NewGetVNetParams().WithVnetID(d.Id())
	v, err := pconf.K.Vnet.GetVNet(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	// set object params
	return vnetToResource(v.Payload, d)
}

func resourceVNetDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := vnet.NewDeleteVNetParams().WithVnetID(d.Id())
	_, err := pconf.K.Vnet.DeleteVNet(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceVNetUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing virtual network
	cfg := newVNet(d)
	params := vnet.NewUpdateVNetParams().WithVnetID(d.Id()).WithBody(&cfg)
	_, err := pconf.K.Vnet.UpdateVNet(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
