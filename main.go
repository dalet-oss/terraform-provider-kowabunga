package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"time"

	"github.com/dalet-oss/terraform-provider-kowabunga/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

const (
	TerraformRegistryPluginName = "registry.terraform.io/dalet-oss/kowabunga"
)

// Generate the Terraform provider documentation using `tfplugindocs`:
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

var version = "was not built correctly" // set via the Makefile or goreleaser

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: TerraformRegistryPluginName,
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
