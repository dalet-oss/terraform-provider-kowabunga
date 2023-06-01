package kowabunga

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"github.com/dalet-oss/kowabunga-api/client/host"
	"github.com/dalet-oss/kowabunga-api/client/zone"
	"github.com/dalet-oss/kowabunga-api/models"
)

func resourceHost() *schema.Resource {
	return &schema.Resource{
		Create: resourceHostCreate,
		Read:   resourceHostRead,
		Update: resourceHostUpdate,
		Delete: resourceHostDelete,
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
			KeyProtocol: {
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
				Type:     schema.TypeInt,
				Optional: true,
			},
			KeyTlsKey: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyTlsCert: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyTlsCA: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
		},
	}
}

func newHost(d *schema.ResourceData) models.Host {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	return models.Host{
		Name:        name,
		Description: desc,
	}
}

func newHostConfiguration(d *schema.ResourceData) models.HostConfiguration {
	name := d.Get(KeyName).(string)
	desc := d.Get(KeyDesc).(string)
	protocol := d.Get(KeyProtocol).(string)
	address := d.Get(KeyAddress).(string)
	port := d.Get(KeyPort).(int)

	hc := models.HostConfiguration{
		Name:        &name,
		Description: desc,
		Protocol:    &protocol,
		Address:     &address,
		Port:        int64(port),
	}

	if protocol == models.HostConfigurationProtocolTLS {
		key := d.Get(KeyTlsKey).(string)
		cert := d.Get(KeyTlsCert).(string)
		ca := d.Get(KeyTlsCA).(string)
		tls := models.HostConfigurationTLS{
			Key:  &key,
			Cert: &cert,
			Ca:   &ca,
		}
		hc.TLS = &tls
	}

	return hc
}

func hostToResource(h *models.Host, d *schema.ResourceData) error {
	// set object params
	err := d.Set(KeyName, h.Name)
	if err != nil {
		return err
	}
	err = d.Set(KeyDesc, h.Description)
	if err != nil {
		return err
	}
	return nil
}

func resourceHostCreate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// find parent zone
	zoneId, err := zoneIDFromID(d, pconf)
	if err != nil {
		return err
	}

	// create a new host
	h := newHostConfiguration(d)
	params := zone.NewCreateHostParams().WithZoneID(zoneId).WithBody(&h)
	hs, err := pconf.K.Zone.CreateHost(params, nil)
	if err != nil {
		return err
	}

	// set resource ID accordingly
	d.SetId(hs.Payload.ID)

	return nil
}

func resourceHostRead(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := host.NewGetHostParams().WithHostID(d.Id())
	h, err := pconf.K.Host.GetHost(params, nil)
	if err != nil {
		return err
	}

	// set object params
	return hostToResource(h.Payload, d)
}

func resourceHostDelete(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	params := host.NewDeleteHostParams().WithHostID(d.Id())
	_, err := pconf.K.Host.DeleteHost(params, nil)
	if err != nil {
		return err
	}

	return nil
}

func resourceHostUpdate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)

	pconf.Mutex.Lock()
	defer pconf.Mutex.Unlock()

	// update an existing region
	h := newHost(d)
	params := host.NewUpdateHostParams().WithHostID(d.Id()).WithBody(&h)
	_, err := pconf.K.Host.UpdateHost(params, nil)
	if err != nil {
		return err
	}

	return nil
}