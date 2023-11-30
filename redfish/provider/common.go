package provider

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"terraform-provider-redfish/redfish/models"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	datasourceSchema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceSchema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

// this defines the operation being executed on resource via terraform
type operation uint8

const (
	operationRead operation = iota + 1
	operationCreate
	operationUpdate
	operationDelete
	operationImport
	redfishServerMD string = "List of server BMCs and their respective user credentials"
)

// RedfishServerSchema to construct schema of redfish server
func RedfishServerSchema() map[string]resourceSchema.Attribute {
	return map[string]resourceSchema.Attribute{
		"user": resourceSchema.StringAttribute{
			Optional:    true,
			Description: "User name for login",
		},
		"password": resourceSchema.StringAttribute{
			Optional:    true,
			Description: "User password for login",
			Sensitive:   true,
		},
		"endpoint": resourceSchema.StringAttribute{
			Required:    true,
			Description: "Server BMC IP address or hostname",
		},
		"ssl_insecure": resourceSchema.BoolAttribute{
			Optional:    true,
			Description: "This field indicates whether the SSL/TLS certificate must be verified or not",
		},
	}
}

// RedfishServerDatasourceSchema to construct schema of redfish server
func RedfishServerDatasourceSchema() map[string]datasourceSchema.Attribute {
	return map[string]datasourceSchema.Attribute{
		"user": datasourceSchema.StringAttribute{
			Optional:    true,
			Description: "User name for login",
		},
		"password": datasourceSchema.StringAttribute{
			Optional:    true,
			Description: "User password for login",
			Sensitive:   true,
		},
		"endpoint": datasourceSchema.StringAttribute{
			Required:    true,
			Description: "Server BMC IP address or hostname",
		},
		"ssl_insecure": datasourceSchema.BoolAttribute{
			Optional:    true,
			Description: "This field indicates whether the SSL/TLS certificate must be verified or not",
		},
	}
}

// RedfishServerResourceBlockMap to construct common block map for data sources
func RedfishServerResourceBlockMap() map[string]resourceSchema.Block {
	return map[string]resourceSchema.Block{
		"redfish_server": resourceSchema.ListNestedBlock{
			MarkdownDescription: redfishServerMD,
			Description:         redfishServerMD,
			Validators: []validator.List{
				listvalidator.SizeAtMost(1),
				listvalidator.IsRequired(),
			},
			NestedObject: resourceSchema.NestedBlockObject{
				Attributes: RedfishServerSchema(),
			},
		},
	}
}

// RedfishServerDatasourceBlockMap to construct common block map for data sources
func RedfishServerDatasourceBlockMap() map[string]datasourceSchema.Block {
	return map[string]datasourceSchema.Block{
		"redfish_server": datasourceSchema.ListNestedBlock{
			MarkdownDescription: redfishServerMD,
			Description:         redfishServerMD,
			Validators: []validator.List{
				listvalidator.SizeAtMost(1),
				listvalidator.IsRequired(),
			},
			NestedObject: datasourceSchema.NestedBlockObject{
				Attributes: RedfishServerDatasourceSchema(),
			},
		},
	}
}

// Based on an instance of Service from the gofish library, retrieve a concrete system on which we can take action
func getSystemResource(service *gofish.Service) (*redfish.ComputerSystem, error) {
	systems, err := service.Systems()
	if err != nil {
		return nil, err
	}
	if len(systems) == 0 {
		return nil, errors.New("no computer systems found")
	}

	return systems[0], err
}

// NewConfig function creates the needed gofish structs to query the redfish API
// See https://github.com/stmcginnis/gofish for details. This function returns a Service struct which can then be
// used to make any required API calls.
// To-Do: Verify from plan modifier, if required implement wrapper for validation of unknown in redfish_server.
func NewConfig(pconfig *redfishProvider, rserver *[]models.RedfishServer) (*gofish.Service, error) {
	if len(*rserver) == 0 {
		return nil, fmt.Errorf("No provider block was found")
	}

	// first redfish server block
	if len(*rserver) == 0 {
		return nil, errors.New("redfish server config not present")
	}
	rserver1 := (*rserver)[0]
	var redfishClientUser, redfishClientPass string

	if len(rserver1.User.ValueString()) > 0 {
		redfishClientUser = rserver1.User.ValueString()
	} else if len(pconfig.Username) > 0 {
		redfishClientUser = pconfig.Username
	} else {
		return nil, fmt.Errorf("error. Either provide username at provider level or resource level. Please check your configuration")
	}

	if len(rserver1.Password.ValueString()) > 0 {
		redfishClientPass = rserver1.Password.ValueString()
	} else if len(pconfig.Password) > 0 {
		redfishClientPass = pconfig.Password
	} else {
		return nil, fmt.Errorf("error. Either provide password at provider level or resource level. Please check your configuration")
	}

	if len(redfishClientUser) == 0 || len(redfishClientPass) == 0 {
		return nil, fmt.Errorf("error. Either Redfish client username or password has not been set. Please check your configuration")
	}

	clientConfig := gofish.ClientConfig{
		Endpoint:  rserver1.Endpoint.ValueString(),
		Username:  redfishClientUser,
		Password:  redfishClientPass,
		BasicAuth: true,
		Insecure:  rserver1.SslInsecure.ValueBool(),
	}
	api, err := gofish.Connect(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error connecting to redfish API: %v", err)
	}
	log.Printf("Connection with the redfish endpoint %v was sucessful\n", rserver1.Endpoint.ValueString())
	return api.Service, nil
}

type powerOperator struct {
	ctx     context.Context
	service *gofish.Service
}

// PowerOperation Executes a power operation against the target server. It takes four arguments. The first is the reset
// type. See the struct "ResetType" at https://github.com/stmcginnis/gofish/blob/main/redfish/computersystem.go for all
// possible options. The second is maximumWaitTime which is the maximum amount of time to wait for the server to reach
// the expected power state before considering it a failure. The third is checkInterval which is how often to check the
// server's power state for updates. The last is a pointer to a gofish.Service object with which the function can
// interact with the server. It will return a tuple consisting of the server's power state at time of return and
// diagnostics
func (p powerOperator) PowerOperation(resetType string, maximumWaitTime int64, checkInterval int64) (redfish.PowerState, error) {
	const powerON redfish.PowerState = "On"
	const powerOFF redfish.PowerState = "Off"
	system, err := getSystemResource(p.service)
	if err != nil {
		tflog.Error(p.ctx, fmt.Sprintf("Failed to identify system: %s", err))
		return "", fmt.Errorf("failed to identify system: %w", err)
	}

	var targetPowerState redfish.PowerState

	if resetType == "ForceOff" || resetType == "GracefulShutdown" {
		if system.PowerState == powerOFF {
			tflog.Trace(p.ctx, "Server already powered off. No action required.")
			return redfish.OffPowerState, nil
		}
		targetPowerState = powerOFF
	}

	if resetType == "On" || resetType == "ForceOn" {
		if system.PowerState == powerON {
			tflog.Trace(p.ctx, "Server already powered on. No action required")
			return redfish.OnPowerState, nil
		}
		targetPowerState = powerON
	}

	if resetType == "ForceRestart" || resetType == "GracefulRestart" || resetType == "PowerCycle" || resetType == "Nmi" {
		// If someone asks for a reset while the server is off, change the reset type to on instead
		if system.PowerState == powerOFF {
			resetType = "On"
		} else {
			targetPowerState = powerON
		}
	}

	if resetType == "PushPowerButton" {
		// In case of Push Power button toggle the current state
		if system.PowerState == powerOFF {
			targetPowerState = powerON
		} else {
			targetPowerState = powerOFF
		}
	}

	// Run the power operation against the target server
	tflog.Trace(p.ctx, fmt.Sprintf("Performing system.Reset(%s)", resetType))
	if err = system.Reset(redfish.ResetType(resetType)); err != nil {
		tflog.Warn(p.ctx, fmt.Sprintf("system.Reset returned an error: %s", err))
		return system.PowerState, err
	}

	// Wait for the server to be in the correct power state
	var totalTime int64
	for totalTime < maximumWaitTime {
		time.Sleep(time.Duration(checkInterval) * time.Second)
		totalTime += checkInterval
		tflog.Trace(p.ctx, fmt.Sprintf("Total time is %d seconds. Checking power state now.", totalTime))

		system, err := getSystemResource(p.service)
		if err != nil {
			tflog.Error(p.ctx, fmt.Sprintf("Failed to identify system: %s", err))
			return targetPowerState, err
		}
		if system.PowerState == targetPowerState {
			tflog.Debug(p.ctx, "system.Reset successful")
			return system.PowerState, nil
		}
	}

	// If we've reached here it means the system never reached the appropriate target state
	// We will instead set the power state to whatever the current state is and return
	// TODO : Change to warning when updated to plugin framework
	tflog.Warn(p.ctx, "The system failed to update the server's power status within the maximum wait time specified!")
	return system.PowerState, nil
}

// checkServerStatus checks iDRAC server status after provided interval until the provided timeout time
func checkServerStatus(ctx context.Context, endpoint string, interval int, timeout int) error {
	var err error
	addr, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	// Intial sleep until iDRAC reboot is triggered
	time.Sleep(30 * time.Second)

	for start := time.Now(); time.Since(start) < (time.Duration(timeout) * time.Second); {
		tflog.Trace(ctx, "Checking server status...")
		time.Sleep(time.Duration(interval) * time.Second)
		_, err = net.Dial("tcp", net.JoinHostPort(addr.Hostname(), addr.Scheme))
		if err == nil {
			return nil
		}
		errctx := tflog.SetField(ctx, "error", err.Error())
		tflog.Trace(errctx, "Site unreachable")
	}

	return err
}