package provider

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const (
	ValidatorNetworkProtocolDescription    = "Protocol must be one of 'udp, 'tcp'"
	ValidatorNetworkProtocolErrUnsupported = "Unsupported protocol"
)

var networkSupportedProtocols = []string{
	"tcp",
	"udp",
}

type stringNetworkProtocolValidator struct{}

func (v stringNetworkProtocolValidator) Description(ctx context.Context) string {
	return ValidatorNetworkProtocolDescription
}

func (v stringNetworkProtocolValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v stringNetworkProtocolValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {

	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	protocol := req.ConfigValue.ValueString()
	if !slices.Contains(networkSupportedProtocols, protocol) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			ValidatorNetworkProtocolErrUnsupported,
			fmt.Sprintf("%s: %s", ValidatorNetworkProtocolErrUnsupported, protocol),
		)
	}
}
