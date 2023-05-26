package kowabunga

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/dalet-oss/kowabunga-api/client/region"
)

const (
	KeyName   = "name"
	KeyDesc   = "desc"
	KeyRegion = "region"
)

const (
	ErrorUnknownRegion = "Unknown region"
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
