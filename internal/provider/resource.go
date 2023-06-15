package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"github.com/dalet-oss/kowabunga-api/client/pool"
	"github.com/dalet-oss/kowabunga-api/client/project"
	"github.com/dalet-oss/kowabunga-api/client/region"
	"github.com/dalet-oss/kowabunga-api/client/subnet"
	"github.com/dalet-oss/kowabunga-api/client/template"
	"github.com/dalet-oss/kowabunga-api/client/vnet"
	"github.com/dalet-oss/kowabunga-api/client/zone"
)

const (
	KeyID           = "id"
	KeyURI          = "uri"
	KeyName         = "name"
	KeyDesc         = "desc"
	KeyRegion       = "region"
	KeyZone         = "zone"
	KeyToken        = "token"
	KeyProtocol     = "protocol"
	KeyAddress      = "address"
	KeyPort         = "port"
	KeyTlsKey       = "key"
	KeyTlsCert      = "cert"
	KeyTlsCA        = "ca"
	KeyPrice        = "price"
	KeyCurrency     = "currency"
	KeyPool         = "pool"
	KeySecret       = "secret"
	KeyVLAN         = "vlan"
	KeyInterface    = "interface"
	KeyPrivate      = "private"
	KeyDefault      = "default"
	KeySubnet       = "subnet"
	KeyCIDR         = "cidr"
	KeyGateway      = "gateway"
	KeyDNS          = "dns"
	KeyDHCP         = "dhcp"
	KeyFirst        = "first"
	KeyLast         = "last"
	KeyVNet         = "vnet"
	KeyMAC          = "hwaddress"
	KeyAddresses    = "addresses"
	KeyReserved     = "reserved"
	KeyTags         = "tags"
	KeyMetadata     = "metadata"
	KeyMaxInstances = "max_instances"
	KeyMaxMemory    = "max_memory"
	KeyMaxStorage   = "max_storage"
	KeyMaxVCPUs     = "max_vcpus"
	KeyProject      = "project"
	KeyType         = "type"
	KeyOS           = "os"
	KeyTemplate     = "template"
	KeySize         = "size"
	KeyResizable    = "resizable"
)

const (
	HelperGbToBytes = 1063256064
)

const (
	ErrorGeneric              = "Kowabunga Error"
	ErrorUnconfiguredResource = "Unexpected Resource Configure Type"
	ErrorExpectedProviderData = "Expected *KowabungaProviderData, got: %T."
	ErrorUnknownRegion        = "Unknown region"
	ErrorUnknownZone          = "Unknown zone"
	ErrorUnknownVNet          = "Unknown virtual network"
	ErrorUnknownSubnet        = "Unknown subnet"
	ErrorUnknownProject       = "Unknown project"
	ErrorUnknownPool          = "Unknown storage pool"
	ErrorUnknownTemplate      = "Unknown volume template"
)

const (
	ResourceIdDescription   = "Resource object internal identifier"
	ResourceNameDescription = "Resource name"
	ResourceDescDescription = "Resource extended description"
)

func errorUnconfiguredResource(req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	resp.Diagnostics.AddError(
		ErrorUnconfiguredResource,
		fmt.Sprintf(ErrorExpectedProviderData, req.ProviderData),
	)
}

func errorCreateGeneric(resp *resource.CreateResponse, err error) {
	resp.Diagnostics.AddError(ErrorGeneric, err.Error())
}

func errorReadGeneric(resp *resource.ReadResponse, err error) {
	resp.Diagnostics.AddError(ErrorGeneric, err.Error())
}

func errorUpdateGeneric(resp *resource.UpdateResponse, err error) {
	resp.Diagnostics.AddError(ErrorGeneric, err.Error())
}

func errorDeleteGeneric(resp *resource.DeleteResponse, err error) {
	resp.Diagnostics.AddError(ErrorGeneric, err.Error())
}

func resourceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		KeyID: schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: ResourceIdDescription,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		KeyName: schema.StringAttribute{
			MarkdownDescription: ResourceNameDescription,
			Required:            true,
		},
		KeyDesc: schema.StringAttribute{
			MarkdownDescription: ResourceDescDescription,
			Optional:            true,
		},
	}
}

func resourceMetadata(req resource.MetadataRequest, resp *resource.MetadataResponse, name string) {
	resp.TypeName = req.ProviderTypeName + "_" + name
}

func resourceImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(KeyID), req, resp)
}

func resourceConfigure(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *KowabungaProviderData {
	if req.ProviderData == nil {
		return nil
	}

	kd, ok := req.ProviderData.(*KowabungaProviderData)
	if !ok {
		errorUnconfiguredResource(req, resp)
		return nil
	}

	return kd
}

func getRegionID(data *KowabungaProviderData, id string) (string, error) {
	// let's suppose param is a proper region ID
	p1 := region.NewGetRegionParams().WithRegionID(id)
	r, err := data.K.Region.GetRegion(p1, nil)
	if err == nil {
		return r.Payload.ID, nil
	}

	// fall back, it may be a region name then, finds its associated ID
	p2 := region.NewGetAllRegionsParams()
	regions, err := data.K.Region.GetAllRegions(p2, nil)
	if err == nil {
		for _, rg := range regions.Payload {
			p := region.NewGetRegionParams().WithRegionID(rg)
			r, err := data.K.Region.GetRegion(p, nil)
			if err == nil && *r.Payload.Name == id {
				return r.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownRegion)
}

func getZoneID(data *KowabungaProviderData, id string) (string, error) {
	// let's suppose param is a proper zone ID
	p1 := zone.NewGetZoneParams().WithZoneID(id)
	z, err := data.K.Zone.GetZone(p1, nil)
	if err == nil {
		return z.Payload.ID, nil
	}

	// fall back, it may be a zone name then, finds its associated ID
	p2 := zone.NewGetAllZonesParams()
	zones, err := data.K.Zone.GetAllZones(p2, nil)
	if err == nil {
		for _, zn := range zones.Payload {
			p := zone.NewGetZoneParams().WithZoneID(zn)
			z, err := data.K.Zone.GetZone(p, nil)
			if err == nil && *z.Payload.Name == id {
				return z.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownZone)
}

func getVNetID(data *KowabungaProviderData, id string) (string, error) {
	// let's suppose param is a proper virtual network ID
	p1 := vnet.NewGetVNetParams().WithVnetID(id)
	v, err := data.K.Vnet.GetVNet(p1, nil)
	if err == nil {
		return v.Payload.ID, nil
	}

	// fall back, it may be a virtual network name then, finds its associated ID
	p2 := vnet.NewGetAllVNetsParams()
	vnets, err := data.K.Vnet.GetAllVNets(p2, nil)
	if err == nil {
		for _, vn := range vnets.Payload {
			p := vnet.NewGetVNetParams().WithVnetID(vn)
			v, err := data.K.Vnet.GetVNet(p, nil)
			if err == nil && *v.Payload.Name == id {
				return v.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownVNet)
}

func getSubnetID(data *KowabungaProviderData, id string) (string, error) {
	// let's suppose param is a proper subnet ID
	p1 := subnet.NewGetSubnetParams().WithSubnetID(id)
	s, err := data.K.Subnet.GetSubnet(p1, nil)
	if err == nil {
		return s.Payload.ID, nil
	}

	// fall back, it may be a subnet name then, finds its associated ID
	p2 := subnet.NewGetAllSubnetsParams()
	subnets, err := data.K.Subnet.GetAllSubnets(p2, nil)
	if err == nil {
		for _, sn := range subnets.Payload {
			p := subnet.NewGetSubnetParams().WithSubnetID(sn)
			s, err := data.K.Subnet.GetSubnet(p, nil)
			if err == nil && *s.Payload.Name == id {
				return s.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownSubnet)
}

func getProjectID(data *KowabungaProviderData, id string) (string, error) {
	// let's suppose param is a proper project ID
	p1 := project.NewGetProjectParams().WithProjectID(id)
	p, err := data.K.Project.GetProject(p1, nil)
	if err == nil {
		return p.Payload.ID, nil
	}

	// fall back, it may be a project name then, finds its associated ID
	p2 := project.NewGetAllProjectsParams()
	projects, err := data.K.Project.GetAllProjects(p2, nil)
	if err == nil {
		for _, pn := range projects.Payload {
			p := project.NewGetProjectParams().WithProjectID(pn)
			prj, err := data.K.Project.GetProject(p, nil)
			if err == nil && *prj.Payload.Name == id {
				return prj.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownProject)
}

func getPoolID(data *KowabungaProviderData, id string) (string, error) {
	// let's suppose param is a proper pool ID
	p1 := pool.NewGetPoolParams().WithPoolID(id)
	p, err := data.K.Pool.GetPool(p1, nil)
	if err == nil {
		return p.Payload.ID, nil
	}

	// fall back, it may be a pool name then, finds its associated ID
	p2 := pool.NewGetAllPoolsParams()
	pools, err := data.K.Pool.GetAllPools(p2, nil)
	if err == nil {
		for _, pn := range pools.Payload {
			p := pool.NewGetPoolParams().WithPoolID(pn)
			pl, err := data.K.Pool.GetPool(p, nil)
			if err == nil && *pl.Payload.Name == id {
				return pl.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownPool)
}

func getTemplateID(data *KowabungaProviderData, id string) (string, error) {
	// let's suppose param is a proper template ID
	p1 := template.NewGetTemplateParams().WithTemplateID(id)
	t, err := data.K.Template.GetTemplate(p1, nil)
	if err == nil {
		return t.Payload.ID, nil
	}

	// fall back, it may be a template name then, finds its associated ID
	p2 := template.NewGetAllTemplatesParams()
	templates, err := data.K.Template.GetAllTemplates(p2, nil)
	if err == nil {
		for _, tn := range templates.Payload {
			p := template.NewGetTemplateParams().WithTemplateID(tn)
			t, err := data.K.Template.GetTemplate(p, nil)
			if err == nil && *t.Payload.Name == id {
				return t.Payload.ID, nil
			}
		}
	}

	return "", fmt.Errorf(ErrorUnknownTemplate)
}
