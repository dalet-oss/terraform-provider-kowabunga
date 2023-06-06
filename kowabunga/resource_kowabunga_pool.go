package kowabunga

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/pool"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"
)

func resourcePool() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePoolCreate,
		ReadContext:   resourcePoolRead,
		UpdateContext: resourcePoolUpdate,
		DeleteContext: resourcePoolDelete,
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
			KeyPool: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyAddress: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyPort: {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IsPortNumber,
			},
			KeySecret: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyDefault: {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func newPool(d *schema.ResourceData) models.StoragePool {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	pool := d.Get(KeyPool).(string)
	address := d.Get(KeyAddress).(string)
	port := int64(d.Get(KeyPort).(int))
	secret := d.Get(KeySecret).(string)

	return models.StoragePool{
		Name:        &name,
		Description: desc,
		Pool:        &pool,
		Address:     &address,
		Port:        &port,
		SecretUUID:  secret,
	}
}

func poolToResource(p *models.StoragePool, d *schema.ResourceData) diag.Diagnostics {
	// set object params
	err := d.Set(KeyName, *p.Name)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyDesc, p.Description)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyPool, *p.Pool)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyAddress, *p.Address)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeyPort, *p.Port)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(KeySecret, p.SecretUUID)
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourcePoolCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// find parent zone
	zoneId, err := zoneIDFromID(d, pconf)
	if err != nil {
		return diag.FromErr(err)
	}

	// create a new pool
	cfg := newPool(d)
	params := zone.NewCreatePoolParams().WithZoneID(zoneId).WithBody(&cfg)
	p, err := pconf.K.Zone.CreatePool(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	// set resource ID accordingly
	d.SetId(p.Payload.ID)

	// set pool as default
	dflt := d.Get(KeyDefault).(bool)
	if dflt {
		params2 := zone.NewUpdateZoneDefaultPoolParams().WithZoneID(zoneId).WithPoolID(p.Payload.ID)
		_, err = pconf.K.Zone.UpdateZoneDefaultPool(params2, nil)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourcePoolRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := pool.NewGetPoolParams().WithPoolID(d.Id())
	p, err := pconf.K.Pool.GetPool(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	// set object params
	return poolToResource(p.Payload, d)
}

func resourcePoolDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := pool.NewDeletePoolParams().WithPoolID(d.Id())
	_, err := pconf.K.Pool.DeletePool(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourcePoolUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing region
	cfg := newPool(d)
	params := pool.NewUpdatePoolParams().WithPoolID(d.Id()).WithBody(&cfg)
	_, err := pconf.K.Pool.UpdatePool(params, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
