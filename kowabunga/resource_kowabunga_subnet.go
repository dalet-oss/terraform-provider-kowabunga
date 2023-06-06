package kowabunga

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/subnet"
	"github.com/dalet-oss/kowabunga-api/client/vnet"
	"github.com/dalet-oss/kowabunga-api/models"
)

func resourceSubnet() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSubnetCreate,
		ReadContext:   resourceSubnetRead,
		UpdateContext: resourceSubnetUpdate,
		DeleteContext: resourceSubnetDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			KeyVNet: {
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
			KeyDefault: {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func newSubnet(d *schema.ResourceData) models.Subnet {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	cidr := d.Get(KeyCIDR).(string)
	gw := d.Get(KeyGateway).(string)
	dns := d.Get(KeyDNS).(string)

	return models.Subnet{
		Name:        &name,
		Description: desc,
		Cidr:        &cidr,
		Gateway:     &gw,
		DNS:         dns,
	}
}

func subnetToResource(s *models.Subnet, d *schema.ResourceData) diag.Diagnostics {
	// set object params
	err := d.Set(KeyName, *s.Name)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyDesc, s.Description)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyCIDR, *s.Cidr)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyGateway, *s.Gateway)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyDNS, s.DNS)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceSubnetCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// find parent vnet
	vnetId, err := vnetIDFromID(d, pconf)
	if err != nil {
		return diag.FromErr(err)
	}

	// create a new subnet
	cfg := newSubnet(d)
	params := vnet.NewCreateSubnetParams().WithVnetID(vnetId).WithBody(&cfg)
	s, err := pconf.K.Vnet.CreateSubnet(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	// set resource ID accordingly
	d.SetId(s.Payload.ID)

	// set subnet as default
	dflt := d.Get(KeyDefault).(bool)
	if dflt {
		params2 := vnet.NewUpdateVNetDefaultSubnetParams().WithVnetID(vnetId).WithSubnetID(s.Payload.ID)
		_, err = pconf.K.Vnet.UpdateVNetDefaultSubnet(params2, nil)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceSubnetRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := subnet.NewGetSubnetParams().WithSubnetID(d.Id())
	s, err := pconf.K.Subnet.GetSubnet(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	// set object params
	return subnetToResource(s.Payload, d)
}

func resourceSubnetDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := subnet.NewDeleteSubnetParams().WithSubnetID(d.Id())
	_, err := pconf.K.Subnet.DeleteSubnet(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceSubnetUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing subnet
	cfg := newSubnet(d)
	params := subnet.NewUpdateSubnetParams().WithSubnetID(d.Id()).WithBody(&cfg)
	_, err := pconf.K.Subnet.UpdateSubnet(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
