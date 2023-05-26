package kowabunga

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/dalet-oss/kowabunga-api/client/region"
	"github.com/dalet-oss/kowabunga-api/client/zone"
)

const (
	KeyName     = "name"
	KeyDesc     = "desc"
	KeyRegion   = "region"
	KeyZone     = "zone"
	KeyProtocol = "protocol"
	KeyAddress  = "address"
	KeyPort     = "port"
	KeyTlsKey   = "key"
	KeyTlsCert  = "cert"
	KeyTlsCA    = "ca"
)

const (
	ErrorUnknownRegion = "Unknown region"
	ErrorUnknownZone   = "Unknown zone"
)

func regionIDFromID(d *schema.ResourceData, pconf *ProviderConfiguration) (string, error) {
	id := d.Get(KeyRegion).(string)

	// let's suppose param is a proper region ID
	p1 := region.NewGetRegionParams().WithRegionID(id)
	r, err := pconf.K.Region.GetRegion(p1, nil)
	if err == nil {
		return r.Payload.ID, nil
	}

	// fall back, it may be a region name then, finds its associated ID
	p2 := region.NewGetAllRegionsParams()
	regions, err := pconf.K.Region.GetAllRegions(p2, nil)
	if err == nil {
		for _, rg := range regions.Payload {
			p := region.NewGetRegionParams().WithRegionID(rg)
			r, err := pconf.K.Region.GetRegion(p, nil)
			if err == nil && r.Payload.Name == id {
				return r.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownRegion)
}

func zoneIDFromID(d *schema.ResourceData, pconf *ProviderConfiguration) (string, error) {
	id := d.Get(KeyZone).(string)

	// let's suppose param is a proper zone ID
	p1 := zone.NewGetZoneParams().WithZoneID(id)
	z, err := pconf.K.Zone.GetZone(p1, nil)
	if err == nil {
		return z.Payload.ID, nil
	}

	// fall back, it may be a zone name then, finds its associated ID
	p2 := zone.NewGetAllZonesParams()
	zones, err := pconf.K.Zone.GetAllZones(p2, nil)
	if err == nil {
		for _, zn := range zones.Payload {
			p := zone.NewGetZoneParams().WithZoneID(zn)
			z, err := pconf.K.Zone.GetZone(p, nil)
			if err == nil && z.Payload.Name == id {
				return z.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownZone)
}
