package kowabunga

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/dalet-oss/kowabunga-api/client"
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

const (
	// MimeJSON is JSON MIME-type representation
	MimeJSON = "application/json"

	// KeyKowabungaProviderURI is the full URI to Kowabunga API server
	KeyKowabungaProviderURI = "uri"
	// KeyKowabungaProviderToken is the API key to authenticate with
	KeyKowabungaProviderToken = "token"
)

// ProviderConfiguration struct for kowabunga-provider
type ProviderConfiguration struct {
	K     *client.Kowabunga
	Mutex *sync.Mutex
	Cond  *sync.Cond
}

// Provider Kowabunga
func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			KeyKowabungaProviderURI: {
				Type:         schema.TypeString,
				Required:     true,
				DefaultFunc:  schema.EnvDefaultFunc("KOWABUNGA", nil),
				ValidateFunc: validation.IsURLWithHTTPorHTTPS,
				Description:  "Kowabunga platform URI",
			},
			KeyKowabungaProviderToken: {
				Type:         schema.TypeString,
				Required:     true,
				DefaultFunc:  schema.EnvDefaultFunc("KOWABUNGA_TOKEN", nil),
				ValidateFunc: validation.All(validation.StringIsNotEmpty),
				Description:  "Kowabunga platform token (API key)",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"kowabunga_region": resourceRegion(),
			"kowabunga_zone":   resourceZone(),
			"kowabunga_netgw":  resourceNetGW(),
			"kowabunga_host":   resourceHost(),
			"kowabunga_pool":   resourcePool(),
			"kowabunga_vnet":   resourceVNet(),
			"kowabunga_subnet": resourceSubnet(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func newKowabungaClient(uri, token string) (*client.Kowabunga, error) {
	if uri == "" || token == "" {
		return nil, fmt.Errorf("The Kowabunga provider needs proper initialization parameters")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	r := httptransport.New(u.Host, client.DefaultBasePath, []string{u.Scheme})
	r.SetDebug(false)
	r.Consumers[MimeJSON] = runtime.JSONConsumer()
	r.Producers[MimeJSON] = runtime.JSONProducer()
	auths := []runtime.ClientAuthInfoWriter{
		httptransport.APIKeyAuth("x-token", "header", token),
	}
	r.DefaultAuthentication = httptransport.Compose(auths...)

	return client.New(r, strfmt.Default), nil
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {

	// check for mandatory requirements
	uri := d.Get(KeyKowabungaProviderURI).(string)
	token := d.Get(KeyKowabungaProviderToken).(string)

	k, err := newKowabungaClient(uri, token)
	if err != nil {
		return nil, err
	}

	var mut sync.Mutex
	var provider = ProviderConfiguration{
		K:     k,
		Mutex: &mut,
		Cond:  sync.NewCond(&mut),
	}

	return &provider, nil
}
