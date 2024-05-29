package provider

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	emailverifier "github.com/AfterShip/email-verifier"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const (
	AgentTypeDesc = "Kowabunga remote agent type must be one of the following: "
	UserRoleDesc  = "Kowabunga user role type must be one of the following: "
	UserEmailDesc = "Kowabunga user email is malformed"
)

// Kowabunga User Role Validator
type stringUserRoleValidator struct{}

func (v stringUserRoleValidator) Description(ctx context.Context) string {
	return UserRoleDesc + strings.Join(userSupportedRoles, ", ")
}

func (v stringUserRoleValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v stringUserRoleValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {

	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	if !slices.Contains(userSupportedRoles, req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Unsupported user role",
			fmt.Sprintf("Unsupported user role %s", req.ConfigValue.ValueString()),
		)
		return
	}
}

// Kowabunga User Email Validator
type stringUserEmailValidator struct{}

func (v stringUserEmailValidator) Description(ctx context.Context) string {
	return UserEmailDesc
}

func (v stringUserEmailValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v stringUserEmailValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {

	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	verifier := emailverifier.NewVerifier()
	ret, err := verifier.Verify(req.ConfigValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Unsupported user email",
			fmt.Sprintf("Unsupported user email %s", req.ConfigValue.ValueString()),
		)
		return
	}
	if !ret.Syntax.Valid {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Malformed user email",
			fmt.Sprintf("User email address %s syntax is invalid", req.ConfigValue.ValueString()),
		)
		return
	}
}

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
			fmt.Sprintf("Unsupported agent type %s", req.ConfigValue.ValueString()),
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

// Custom Protocol Validator
type stringProtocolValidator struct{}

func (v stringProtocolValidator) Description(ctx context.Context) string {
	return "Protocol must be one of 'udp, 'tcp'"
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v stringProtocolValidator) MarkdownDescription(ctx context.Context) string {
	return "Protocol must be one of 'udp, 'tcp'"
}
func (v stringProtocolValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {

	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	authorizedProtocols := []string{
		"tcp",
		"udp",
	}

	protocol := req.ConfigValue.ValueString()
	if !slices.Contains(authorizedProtocols, protocol) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Unsupported protocol",
			fmt.Sprintf("Unsupported protocol: %s", protocol),
		)
	}
}
