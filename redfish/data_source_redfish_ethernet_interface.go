package redfish

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceRedfishEthernetInterface() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceRedfishEthernetInterfaceRead,
		Schema:      getDataSourceRedfishEthernetInterfaceSchema(),
	}
}

func getDataSourceRedfishEthernetInterfaceSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"redfish_server": {
			Type:        schema.TypeList,
			Optional:    true,
			Description: "List of server BMCs and their respective user credentials",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"user": {
						Type:        schema.TypeString,
						Optional:    true,
						Description: "User name for login",
					},
					"password": {
						Type:        schema.TypeString,
						Optional:    true,
						Description: "User password for login",
						Sensitive:   true,
					},
					"endpoint": {
						Type:        schema.TypeString,
						Required:    true,
						Description: "Server BMC IP address or hostname",
					},
					"ssl_insecure": {
						Type:        schema.TypeBool,
						Optional:    true,
						Description: "This field indicates whether the SSL/TLS certificate must be verified or not",
					},
				},
			},
		},
		"manager_id": {
			Type:        schema.TypeString,
			Description: "ID of the manager",
			Required:    true,
			Default:     "1",
		},
		"ethernet_interface_id": {
			Type:        schema.TypeString,
			Description: "ID of the ethernet interface",
			Required:    true,
			Default:     "1",
		},
		"dhcpv4_config": {
			Type:        schema.TypeList,
			MinItems:    1,
			MaxItems:    1,
			Optional:    true,
			Description: "DHCPv4 configuration for this interface.",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"dhcp_enabled": {
						Type:        schema.TypeBool,
						Description: "An indication of whether DHCP v4 is enabled on this Ethernet interface.",
						Optional:    true,
					},
					// "fallback_address": {
					// 	Type:        schema.TypeList,
					// 	Description: "DHCPv4 fallback address method for this interface.",
					// 	Required:    false,
					// },
					"use_dns_servers": {
						Type:        schema.TypeBool,
						Description: "An indication of whether this interface uses DHCP v4-supplied DNS servers.",
						Optional:    true,
					},
					"use_domain_name": {
						Type:        schema.TypeBool,
						Description: "An indication of whether this interface uses a DHCP v4-supplied domain name.",
						Optional:    true,
					},
					"use_gateway": {
						Type:        schema.TypeBool,
						Description: "An indication of whether this interface uses a DHCP v4-supplied gateway.",
						Optional:    true,
					},
					"use_ntp_servers": {
						Type:        schema.TypeBool,
						Description: "An indication of whether the interface uses DHCP v4-supplied NTP servers.",
						Optional:    true,
					},
					"use_static_routes": {
						Type:        schema.TypeBool,
						Description: "An indication of whether the interface uses DHCP v4-supplied static routes.",
						Optional:    true,
					},
				},
			},
		},
		"dhcpv6_config": {
			Type:        schema.TypeList,
			MinItems:    1,
			MaxItems:    1,
			Optional:    true,
			Description: "DHCPv6 configuration for this interface.",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"operating_mode": {
						Type:         schema.TypeString,
						Description:  "Determines the DHCPv6 operating mode for this interface.",
						Optional:     true,
						ExactlyOneOf: []string{"Stateful", "Stateless", "Disabled", "Enabled"},
					},
					"use_dns_servers": {
						Type:        schema.TypeBool,
						Description: "An indication of whether the interface uses DHCP v6-supplied DNS servers.",
						Optional:    true,
					},
					"use_domain_name": {
						Type:        schema.TypeBool,
						Description: "An indication of whether this interface uses a DHCP v6-supplied domain name.",
						Optional:    true,
					},
					"use_ntp_servers": {
						Type:        schema.TypeBool,
						Description: "An indication of whether the interface uses DHCP v6-supplied NTP servers.",
						Optional:    true,
					},
					"use_rapid_commit": {
						Type:        schema.TypeBool,
						Description: "An indication of whether the interface uses DHCP v6 rapid commit mode for stateful mode address assignments.  Do not enable this option in networks where more than one DHCP v6 server is configured to provide address assignments.",
						Optional:    true,
					},
				},
			},
		},
	}
}

func dataSourceRedfishEthernetInterfaceRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	service, err := NewConfig(m.(*schema.ResourceData), d)
	if err != nil {
		return diag.Errorf(err.Error())
	}

	ethernetInterface, err := getRedfishEthernetInterface(service, d)
	if err != nil {
		return diag.Errorf("Couldn't get Ethernet Interface: %s", err)
	}

	d.SetId(ethernetInterface.ODataID)

	// Set terraform schema data
	if err := d.Set("dhcp_enabled", ethernetInterface.DHCPv4.DHCPEnabled); err != nil {
		return diag.Errorf("[CUSTOM] error setting %s: %v\n", "dhcp_enabled", err)
	}

	if err := readRedfishEthernetInterfaceDHCPv4Configuration(ethernetInterface, d); err != nil {
		return diag.Errorf("Couldn't read Ethernet Interface DHCPv4 configuration: %s", err)
	}

	if err := readRedfishEthernetInterfaceDHCPv6Configuration(ethernetInterface, d); err != nil {
		return diag.Errorf("Couldn't read Ethernet Interface DHCPv6 configuration: %s", err)
	}

	return nil
}
