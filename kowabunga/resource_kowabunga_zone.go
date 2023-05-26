package kowabunga

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/region"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"
)

func resourceZone() *schema.Resource {
	return &schema.Resource{
		Create: resourceZoneCreate,
		Read:   resourceZoneRead,
		Update: resourceZoneUpdate,
		Delete: resourceZoneDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			KeyRegion: {
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
		},
	}
}

func newZone(d *schema.ResourceData) models.Zone {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	return models.Zone{
		Name:        name,
		Description: desc,
	}
}

func zoneToResource(r *models.Zone, d *schema.ResourceData) error {
	// set object params
	err := d.Set(KeyName, r.Name)
	if err != nil {
		return err
	}
	err = d.Set(KeyDesc, r.Description)
	if err != nil {
		return err
	}
	return nil
}

func resourceZoneCreate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// find parent region
	regionId, err := regionIDFromID(d, pconf)
	if err != nil {
		return err
	}

	// create a new zone
	z := newZone(d)
	params := region.NewCreateZoneParams().WithRegionID(regionId).WithBody(&z)
	zn, err := pconf.K.Region.CreateZone(params, nil)
	if err != nil {
		return err
	}

	// set resource ID accordingly
	d.SetId(zn.Payload.ID)

	return nil
}

func resourceZoneRead(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := zone.NewGetZoneParams().WithZoneID(d.Id())
	z, err := pconf.K.Zone.GetZone(params, nil)
	if err != nil {
		return err
	}

	// set object params
	return zoneToResource(z.Payload, d)
}

func resourceZoneDelete(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := zone.NewDeleteZoneParams().WithZoneID(d.Id())
	_, err := pconf.K.Zone.DeleteZone(params, nil)
	if err != nil {
		return err
	}

	return nil
}

func resourceZoneUpdate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing region
	z := newZone(d)
	params := zone.NewUpdateZoneParams().WithZoneID(d.Id()).WithBody(&z)
	_, err := pconf.K.Zone.UpdateZone(params, nil)
	if err != nil {
		return err
	}

	return nil
}
