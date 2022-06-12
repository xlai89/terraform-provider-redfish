package redfish

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/stmcginnis/gofish/redfish"
)

func resourceRedfishEthernetInterface() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRedfishEthernetInterfaceCreate,
		ReadContext:   resourceRedfishEthernetInterfaceRead,
		UpdateContext: resourceRedfishEthernetInterfaceUpdate,
		DeleteContext: resourceRedfishEthernetInterfaceDelete,
		Schema:        getResourceRedfishEthernetInterfaceSchema(),
	}
}

func getResourceRedfishEthernetInterfaceSchema() map[string]*schema.Schema {
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
		"dhcpv4": {
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
		"dhcpv6": {
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
		"ipv4_static_addresses": {
			Type:        schema.TypeList,
			Optional:    true,
			Description: "The IPv4 static addresses assigned to this interface.",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"address": {
						Type:         schema.TypeString,
						Description:  "The IPv4 address",
						Optional:     true,
						ValidateFunc: validation.StringMatch(regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`), "Not a valid ipv4 address."),
					},
					"address_origin": {
						Type:        schema.TypeString,
						Description: "This indicates how the address was determined.",
						Computed:    true,
					},
					"gateway": {
						Type:         schema.TypeString,
						Description:  "The IPv4 gateway for this address.",
						Optional:     true,
						ValidateFunc: validation.StringMatch(regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`), "Not a valid ipv4 gateway."),
					},
					"subnet_mask": {
						Type:         schema.TypeString,
						Description:  "The IPv4 subnet mask.",
						Optional:     true,
						ValidateFunc: validation.StringMatch(regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`), "Not a valid ipv4 subnet mask."),
					},
				},
			},
		},
	}
}

func resourceRedfishEthernetInterfaceCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	service, err := NewConfig(m.(*schema.ResourceData), d)
	if err != nil {
		return diag.Errorf(err.Error())
	}

	// Lock the mutex to avoid race conditions with other resources
	redfishMutexKV.Lock(getRedfishServerEndpoint(d))
	defer redfishMutexKV.Unlock(getRedfishServerEndpoint(d))

	ethernetInterface, err := getRedfishEthernetInterface(service, d)
	if err != nil {
		return diag.Errorf("Couldn't get Ethernet Interface: %s", err)
	}

	ethernetInterface.DHCPv4 = *createRedfishEthernetInterfaceDHCPv4Configuration(d)

	ethernetInterface.DHCPv6 = *createRedfishEthernetInterfaceDHCPv6Configuration(d)

	ethernetInterface.IPv4StaticAddresses = createRedfishEthernetInterfaceIPv4StaticAddresses(d)

	d.SetId(ethernetInterface.ODataID)

	return resourceRedfishEthernetInterfaceRead(ctx, d, m)
}

func resourceRedfishEthernetInterfaceRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	service, err := NewConfig(m.(*schema.ResourceData), d)
	if err != nil {
		return diag.Errorf(err.Error())
	}

	ethernetInterface, err := redfish.GetEthernetInterface(service.Client, d.Id())
	if err != nil {
		return diag.Errorf("Ethernet Interface doesn't exist: %s", err) //This error won't be triggered ever
	}

	if err := readRedfishEthernetInterfaceDHCPv4Configuration(ethernetInterface, d); err != nil {
		return diag.Errorf("Couldn't read Ethernet Interface DHCPv4 configuration: %s", err)
	}

	if err := readRedfishEthernetInterfaceDHCPv6Configuration(ethernetInterface, d); err != nil {
		return diag.Errorf("Couldn't read Ethernet Interface DHCPv6 configuration: %s", err)
	}

	if err := readRedfishEthernetInterfaceIPv4StaticAddresses(ethernetInterface, d); err != nil {
		return diag.Errorf("Couldn't read Ethernet Interface IPv4 static addresses: %s", err)
	}

	return nil
}

func resourceRedfishEthernetInterfaceUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	service, err := NewConfig(m.(*schema.ResourceData), d)
	if err != nil {
		return diag.Errorf(err.Error())
	}

	// Lock the mutex to avoid race conditions with other resources
	redfishMutexKV.Lock(getRedfishServerEndpoint(d))
	defer redfishMutexKV.Unlock(getRedfishServerEndpoint(d))

	ethernetInterface, err := redfish.GetEthernetInterface(service.Client, d.Id())
	if err != nil {
		return diag.Errorf("Ethernet Interface doesn't exist: %s", err) //This error won't be triggered ever
	}

	if d.HasChange("dhcpv4") {
		ethernetInterface.DHCPv4 = *updateRedfishEthernetInterfaceDHCPv4Configuration(d)
	}

	if d.HasChange("dhcpv6") {
		ethernetInterface.DHCPv6 = *updateRedfishEthernetInterfaceDHCPv6Configuration(d)
	}

	if d.HasChange("ipv4_static_addresses") {
		ethernetInterface.IPv4StaticAddresses = updateRedfishEthernetInterfaceIPv4StaticAddresses(d)
	}

	return resourceRedfishEthernetInterfaceRead(ctx, d, m)
}

func resourceRedfishEthernetInterfaceDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {

	d.SetId("")

	return nil
}
