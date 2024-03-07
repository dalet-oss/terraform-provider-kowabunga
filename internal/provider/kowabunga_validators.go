package provider

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const (
	AgentTypeDesc = "Kowabunga remote agent type must be one of the following: "
)

// Kowabunga Agent Type Validator
type stringAgentTypeValidator struct{}

func (v stringAgentTypeValidator) Description(ctx context.Context) string {
	return AgentTypeDesc + strings.Join(agentSupportedTypes, ", ")
}

func (v stringAgentTypeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v stringAgentTypeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {

	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	if !slices.Contains(agentSupportedTypes, req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Unsupported agent type",
			fmt.Sprintf("Unsupported agent type %s: ", req.ConfigValue.ValueString()),
		)
		return
	}
}

// Custom Port Validator
type stringPortValidator struct{}

func (v stringPortValidator) Description(ctx context.Context) string {
	return "Ports format must follow the following rules : comma seprated, ranges ordered with a \"-\" char. e.g : 1234, 5678-5690"
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v stringPortValidator) MarkdownDescription(ctx context.Context) string {
	return "Ports format must follow the following rules : comma seprated, ranges ordered with a \"-\" char. e.g : 1234, 5678-5690"
}
func (v stringPortValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {

	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	portList := strings.Split(req.ConfigValue.ValueString(), ",")
	for _, port := range portList {
		portRanges := strings.Split(port, "-") //returns at least 1 entry
		if len(portRanges) > 2 {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Too many entries",
				fmt.Sprintf("Too many entries in range %s: ", port),
			)
			return
		}
		_, err := strconv.ParseUint(portRanges[0], 10, 16)
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid port",
				fmt.Sprintf("Port outside range (0-65535) for port  : %s ", portRanges[0]),
			)
			return
		}
		if len(portRanges) == 2 && err == nil {
			_, err = strconv.ParseUint(portRanges[1], 10, 16)
			if err != nil {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid port",
					fmt.Sprintf("Port outside range (0-65535) for port  : %s ", portRanges[1]),
				)
				return
			}
			if portRanges[0] > portRanges[1] {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Range",
					fmt.Sprintf("Left hand side is superior than righ hand side : %s ", port),
				)
				return
			}
		}
	}
}
