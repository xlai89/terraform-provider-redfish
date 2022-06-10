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
	}
}

func dataSourceRedfishEthernetInterfaceRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	service, err := NewConfig(m.(*schema.ResourceData), d)
	if err != nil {
		return diag.Errorf(err.Error())
	}

	// Get manager id and ethernet interface id from schema
	var managerID, ethernetInterfaceID string
	if v, ok := d.GetOk("manager_id"); ok {
		managerID = v.(string)
	}
	if v, ok := d.GetOk("ethernet_interface_id"); ok {
		ethernetInterfaceID = v.(string)
	}

	// Get manager list and then a specific manager
	managerCollection, err := service.Managers()
	if err != nil {
		return diag.Errorf("Couldn't retrieve managers from redfish API: %s", err)
	}
	manager, err := getManager(managerID, managerCollection)
	if err != nil {
		return diag.Errorf("Manager selected doesn't exist: %s", err)
	}

	// Get ethernet interface list and then a specific ethernet interface
	ethernetInterfaceCollection, err := manager.EthernetInterfaces()
	if err != nil {
		return diag.Errorf("Couldn't retrieve ethernet interface collection from redfish API: %s", err)
	}
	ethernetInterface, err := getEthernetInterface(ethernetInterfaceID, ethernetInterfaceCollection)
	if err != nil {
		return diag.Errorf("Ethernet Interface selected doesn't exist: %s", err)
	}

	d.SetId(ethernetInterface.ODataID)

	// Set terraform schema data
	if err := d.Set("dhcp_enabled", ethernetInterface.DHCPv4.DHCPEnabled); err != nil {
		return diag.Errorf("[CUSTOM] error setting %s: %v\n", "dhcp_enabled", err)
	}

	if err := d.Set("use_dns_servers", ethernetInterface.DHCPv4.UseDNSServers); err != nil {
		return diag.Errorf("[CUSTOM] error setting %s: %v\n", "use_dns_servers", err)
	}

	if err := d.Set("use_domain_name", ethernetInterface.DHCPv4.UseDomainName); err != nil {
		return diag.Errorf("[CUSTOM] error setting %s: %v\n", "use_domain_name", err)
	}

	if err := d.Set("use_gateway", ethernetInterface.DHCPv4.UseGateway); err != nil {
		return diag.Errorf("[CUSTOM] error setting %s: %v\n", "use_gateway", err)
	}

	if err := d.Set("use_ntp_servers", ethernetInterface.DHCPv4.UseNTPServers); err != nil {
		return diag.Errorf("[CUSTOM] error setting %s: %v\n", "use_ntp_servers", err)
	}

	if err := d.Set("use_static_routes", ethernetInterface.DHCPv4.UseStaticRoutes); err != nil {
		return diag.Errorf("[CUSTOM] error setting %s: %v\n", "use_static_routes", err)
	}

	return nil
}
