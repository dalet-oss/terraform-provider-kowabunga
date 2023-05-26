package kowabunga

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/region"
	"github.com/dalet-oss/kowabunga-api/models"
)

const (
	KeyName = "name"
	KeyDesc = "desc"
)

func resourceRegion() *schema.Resource {
	return &schema.Resource{
		Create: resourceRegionCreate,
		Read:   resourceRegionRead,
		Update: resourceRegionUpdate,
		Delete: resourceRegionDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
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

func newRegion(d *schema.ResourceData) models.Region {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	return models.Region{
		Name:        name,
		Description: desc,
	}
}

func regionToResource(r *models.Region, d *schema.ResourceData) error {
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

func resourceRegionCreate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// create a new region
	rg := newRegion(d)
	params := region.NewCreateRegionParams().WithBody(&rg)
	r, err := pconf.K.Region.CreateRegion(params, nil)
	if err != nil {
		return err
	}

	// set resource ID accordingly
	d.SetId(r.Payload.ID)

	return nil
}

func resourceRegionRead(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := region.NewGetRegionParams().WithRegionID(d.Id())
	r, err := pconf.K.Region.GetRegion(params, nil)
	if err != nil {
		return err
	}

	// set object params
	return regionToResource(r.Payload, d)
}

func resourceRegionDelete(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := region.NewDeleteRegionParams().WithRegionID(d.Id())
	_, err := pconf.K.Region.DeleteRegion(params, nil)
	if err != nil {
		return err
	}

	return nil
}

func resourceRegionUpdate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing region
	rg := newRegion(d)
	params := region.NewUpdateRegionParams().WithRegionID(d.Id()).WithBody(&rg)
	_, err := pconf.K.Region.UpdateRegion(params, nil)
	if err != nil {
		return err
	}

	return nil
}
