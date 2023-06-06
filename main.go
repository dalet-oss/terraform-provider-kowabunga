package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/dalet-oss/terraform-provider-kowabunga/kowabunga"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

// Generate the Terraform provider documentation using `tfplugindocs`:
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

var version = "was not built correctly" // set via the Makefile

func main() {
	versionFlag := flag.Bool("version", false, "print version information and exit")
	flag.Parse()
	if *versionFlag {
		err := printVersion(os.Stdout)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: kowabunga.Provider,
	})
}

func printVersion(writer io.Writer) error {
	_, err := fmt.Fprintf(writer, "%s %s\n", os.Args[0], version)
	return err
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
