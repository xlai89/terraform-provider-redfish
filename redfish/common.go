package redfish

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

// Based on an instance of Service from the gofish library, retrieve a concrete system on which we can take action
func getSystemResource(service *gofish.Service) (*redfish.ComputerSystem, error) {

	systems, err := service.Systems()

	if err != nil {
		return nil, err
	}
	if len(systems) == 0 {
		return nil, errors.New("No computer systems found")
	}

	return systems[0], err
}

// NewConfig function creates the needed gofish structs to query the redfish API
// See https://github.com/stmcginnis/gofish for details. This function returns a Service struct which can then be
// used to make any required API calls.
func NewConfig(provider *schema.ResourceData, resource *schema.ResourceData) (*gofish.Service, error) {
	//Get redfish connection details from resource block
	var providerUser, providerPassword, providerEndpoint string
	var providerSslInsecure bool

	if v, ok := provider.GetOk("user"); ok {
		providerUser = v.(string)
	}
	if v, ok := provider.GetOk("password"); ok {
		providerPassword = v.(string)
	}
	if v, ok := provider.GetOk("endpoint"); ok {
		providerEndpoint = v.(string)
	}
	if v, ok := provider.GetOk("ssl_insecure"); ok {
		providerSslInsecure = v.(bool)
	}

	resourceServerConfig := resource.Get("redfish_server").([]interface{}) //It must be just one element

	//Overwrite parameters
	//Get redfish username at resource level over provider level
	var redfishClientUser, redfishClientPass, redfishClientEndpoint string
	var redfishClientSslInsecure bool
	if len(resourceServerConfig[0].(map[string]interface{})["user"].(string)) > 0 {
		redfishClientUser = resourceServerConfig[0].(map[string]interface{})["user"].(string)
		log.Println("Using redfish user from resource")
	} else {
		redfishClientUser = providerUser
		log.Println("Using redfish user from provider")
	}
	//Get redfish password at resource level over provider level
	if len(resourceServerConfig[0].(map[string]interface{})["password"].(string)) > 0 {
		redfishClientPass = resourceServerConfig[0].(map[string]interface{})["password"].(string)
		log.Println("Using redfish password from resource")
	} else {
		redfishClientPass = providerPassword
		log.Println("Using redfish password from provider")
	}
	//Get redfish endpoint at resource level over provider level
	if len(resourceServerConfig[0].(map[string]interface{})["endpoint"].(string)) > 0 {
		redfishClientEndpoint = resourceServerConfig[0].(map[string]interface{})["endpoint"].(string)
		log.Println("Using redfish endpoint from resource")
	} else {
		redfishClientEndpoint = providerEndpoint
		log.Println("Using redfish endpoint from provider")
	}
	//Get redfish ssl_insecure at resource level over provider level
	if resourceServerConfig[0].(map[string]interface{})["ssl_insecure"] != nil {
		redfishClientSslInsecure = resourceServerConfig[0].(map[string]interface{})["ssl_insecure"].(bool)
		log.Println("Using redfish ssl_insecure from resource")
	} else {
		redfishClientSslInsecure = providerSslInsecure
		log.Println("Using redfish ssl_insecure from provider")
	}
	//If for some reason none user or pass has been set at provider/resource level, trow an error
	if len(redfishClientUser) == 0 || len(redfishClientPass) == 0 {
		return nil, fmt.Errorf("Error. Either Redfish client username or password has not been set. Please check your configuration")
	}

	clientConfig := gofish.ClientConfig{
		Endpoint:  redfishClientEndpoint,
		Username:  redfishClientUser,
		Password:  redfishClientPass,
		BasicAuth: true,
		Insecure:  redfishClientSslInsecure,
	}
	api, err := gofish.Connect(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("Error connecting to redfish API: %v", err)
	}
	log.Printf("Connection with the redfish endpoint %v was sucessful\n", resourceServerConfig[0].(map[string]interface{})["endpoint"].(string))
	return api.Service, nil
}

// PowerOperation Executes a power operation against the target server. It takes four arguments. The first is the reset
// type. See the struct "ResetType" at https://github.com/stmcginnis/gofish/blob/main/redfish/computersystem.go for all
// possible options. The second is maximumWaitTime which is the maximum amount of time to wait for the server to reach
// the expected power state before considering it a failure. The third is checkInterval which is how often to check the
// server's power state for updates. The last is a pointer to a gofish.Service object with which the function can
// interact with the server. It will return a tuple consisting of the server's power state at time of return and
// diagnostics
func PowerOperation(resetType string, maximumWaitTime int, checkInterval int, service *gofish.Service) (redfish.PowerState, diag.Diagnostics) {

	var diags diag.Diagnostics

	system, err := getSystemResource(service)
	if err != nil {
		log.Printf("[ERROR]: Failed to identify system: %s", err)
		return "", diag.Errorf(err.Error())
	}

	var targetPowerState redfish.PowerState

	if resetType == "ForceOff" || resetType == "GracefulShutdown" {
		if system.PowerState == "Off" {
			log.Printf("[TRACE]: Server already powered off. No action required.")
			return redfish.OffPowerState, diags
		} else {
			targetPowerState = "Off"
		}
	}

	if resetType == "On" || resetType == "ForceOn" {
		if system.PowerState == "On" {
			log.Printf("[TRACE]: Server already powered on. No action required.")
			return redfish.OnPowerState, diags
		} else {
			targetPowerState = "On"
		}
	}

	if resetType == "ForceRestart" || resetType == "GracefulRestart" || resetType == "PowerCycle" {
		// If someone asks for a reset while the server is off, change the reset type to on instead
		if system.PowerState == "Off" {
			resetType = "On"
		}
		targetPowerState = "On"
	}

	// Run the power operation against the target server
	log.Printf("[TRACE]: Performing system.Reset(%s)", resetType)
	if err = system.Reset(redfish.ResetType(resetType)); err != nil {
		log.Printf("[WARN]: system.Reset returned an error: %s", err)
		return system.PowerState, diag.Errorf(err.Error())
	}

	// Wait for the server to be in the correct power state
	totalTime := 0
	for totalTime < maximumWaitTime {

		time.Sleep(time.Duration(checkInterval) * time.Second)
		totalTime += checkInterval
		log.Printf("[TRACE]: Total time is %d seconds. Checking power state now.", totalTime)

		system, err := getSystemResource(service)
		if err != nil {
			log.Printf("[ERROR]: Failed to identify system: %s", err)
			return system.PowerState, diag.Errorf(err.Error())
		}

		if system.PowerState == targetPowerState {
			log.Printf("[TRACE]: system.Reset successful")
			return system.PowerState, diags
		}

	}

	// If we've reached here it means the system never reached the appropriate target state
	// We will instead set the power state to whatever the current state is and return
	log.Printf("[ERROR]: The system failed to correctly update the system power!")
	return system.PowerState, diags

}

// getRedfishServerEndpoint returns the endpoint from an schema. This might be useful
// when using MutexKV, since we need a way to differentiate mutex operations
// across servers
func getRedfishServerEndpoint(resource *schema.ResourceData) string {
	resourceServerConfig := resource.Get("redfish_server").([]interface{})
	return resourceServerConfig[0].(map[string]interface{})["endpoint"].(string)
}

// getManager returns the redfish manager instance in a given manager list
// according to the given manager ID
func getManager(managerID string, mgrs []*redfish.Manager) (*redfish.Manager, error) {
	for _, v := range mgrs {
		if v.ID == managerID {
			return v, nil
		}
	}
	return nil, fmt.Errorf("Manager with ID %s doesn't exist", managerID)
}

// getEthernetInterface return the redfish ethernet interface instance
// in a given ethernet interface list according to the given ethernet interface ID
func getEthernetInterface(ethernetInterfaceID string, ethifs []*redfish.EthernetInterface) (*redfish.EthernetInterface, error) {
	for _, v := range ethifs {
		if v.ID == ethernetInterfaceID {
			return v, nil
		}
	}
	return nil, fmt.Errorf("EthernetInterface with ID %s doesn't exist", ethernetInterfaceID)
}

// getRedfishEthernetInterface return the redfish ethernet interface instance
// according to the manager_id and ethernet_interface_id in the schema
func getRedfishEthernetInterface(service *gofish.Service, d *schema.ResourceData) (*redfish.EthernetInterface, error) {

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
		return nil, fmt.Errorf("Couldn't retrieve managers from redfish API: %s", err)
	}
	manager, err := getManager(managerID, managerCollection)
	if err != nil {
		return nil, fmt.Errorf("Manager selected doesn't exist: %s", err)
	}

	// Get ethernet interface list and then a specific ethernet interface
	ethernetInterfaceCollection, err := manager.EthernetInterfaces()
	if err != nil {
		return nil, fmt.Errorf("Couldn't retrieve ethernet interface collection from redfish API: %s", err)
	}
	ethernetInterface, err := getEthernetInterface(ethernetInterfaceID, ethernetInterfaceCollection)
	if err != nil {
		return nil, fmt.Errorf("Ethernet Interface selected doesn't exist: %s", err)
	}

	return ethernetInterface, nil
}

// createRedfishEthernetInterfaceDHCPv4Configuration turns schema values
// for ethernet interface DHCPv4 configuration into redfish.DHCPv4Configuration
func createRedfishEthernetInterfaceDHCPv4Configuration(d *schema.ResourceData) *redfish.DHCPv4Configuration {

	var dhcpv4ConfigList []interface{}
	if v, ok := d.GetOk("dhcpv4"); ok {
		dhcpv4ConfigList = v.([]interface{})
	}

	if len(dhcpv4ConfigList) > 1 || dhcpv4ConfigList[0] == nil {
		return nil
	}

	dhcpv4Config := dhcpv4ConfigList[0].(map[string]interface{})

	// Get terraform schema data
	var dhcpEnabled, useDnsServers, useDomainName, useGateway, useNTPServers, useStaticRoutes bool
	if v, ok := dhcpv4Config["dhcp_enabled"]; ok {
		dhcpEnabled = v.(bool)
	}
	if v, ok := dhcpv4Config["use_dns_servers"]; ok {
		useDnsServers = v.(bool)
	}
	if v, ok := dhcpv4Config["use_domain_name"]; ok {
		useDomainName = v.(bool)
	}
	if v, ok := dhcpv4Config["use_gateway"]; ok {
		useGateway = v.(bool)
	}
	if v, ok := dhcpv4Config["use_ntp_servers"]; ok {
		useNTPServers = v.(bool)
	}
	if v, ok := dhcpv4Config["use_static_routes"]; ok {
		useStaticRoutes = v.(bool)
	}

	dhcpv4Configuration := redfish.DHCPv4Configuration{
		DHCPEnabled:     dhcpEnabled,
		UseDNSServers:   useDnsServers,
		UseDomainName:   useDomainName,
		UseGateway:      useGateway,
		UseNTPServers:   useNTPServers,
		UseStaticRoutes: useStaticRoutes,
	}

	return &dhcpv4Configuration
}

// readRedfishEthernetInterfaceDHCPv4Configuration takes DHCPv4 configurations
// of the ethernet interface from the redfish client and save them into the terraform schema
func readRedfishEthernetInterfaceDHCPv4Configuration(ethernetInterface *redfish.EthernetInterface, d *schema.ResourceData) error {

	dhcpv4Config := map[string]interface{}{}

	dhcpv4Config["dhcp_enabled"] = ethernetInterface.DHCPv4.DHCPEnabled
	dhcpv4Config["use_dns_servers"] = ethernetInterface.DHCPv4.UseDNSServers
	dhcpv4Config["use_domain_name"] = ethernetInterface.DHCPv4.UseDomainName
	dhcpv4Config["use_gateway"] = ethernetInterface.DHCPv4.UseGateway
	dhcpv4Config["use_ntp_servers"] = ethernetInterface.DHCPv4.UseNTPServers
	dhcpv4Config["use_static_routes"] = ethernetInterface.DHCPv4.UseStaticRoutes

	// Set terraform schema data
	if err := d.Set("dhcpv4", dhcpv4Config); err != nil {
		return fmt.Errorf("[CUSTOM] error setting %s: %v\n", "dhcpv4", err)
	}

	return nil
}

func updateRedfishEthernetInterfaceDHCPv4Configuration(d *schema.ResourceData) *redfish.DHCPv4Configuration {
	return createRedfishEthernetInterfaceDHCPv4Configuration(d)
}

// createRedfishEthernetInterfaceDHCPv6Configuration turns schema values
// for ethernet interface DHCPv4 configuration into redfish.DHCPv4Configuration
func createRedfishEthernetInterfaceDHCPv6Configuration(d *schema.ResourceData) *redfish.DHCPv6Configuration {

	var dhcpv6ConfigList []interface{}
	if v, ok := d.GetOk("dhcpv6"); ok {
		dhcpv6ConfigList = v.([]interface{})
	}

	if len(dhcpv6ConfigList) > 1 || dhcpv6ConfigList[0] == nil {
		return nil
	}

	dhcpv6Config := dhcpv6ConfigList[0].(map[string]interface{})

	// Get terraform schema data
	var operatingMode redfish.DHCPv6OperatingMode
	var useDnsServers, useDomainName, useNTPServers, useRapidCommit bool
	if v, ok := dhcpv6Config["operating_mode"]; ok {
		operatingMode = v.(redfish.DHCPv6OperatingMode)
	}
	if v, ok := dhcpv6Config["use_dns_servers"]; ok {
		useDnsServers = v.(bool)
	}
	if v, ok := dhcpv6Config["use_domain_name"]; ok {
		useDomainName = v.(bool)
	}
	if v, ok := dhcpv6Config["use_ntp_servers"]; ok {
		useNTPServers = v.(bool)
	}
	if v, ok := dhcpv6Config["use_rapid_commit"]; ok {
		useRapidCommit = v.(bool)
	}

	dhcpv6Configuration := redfish.DHCPv6Configuration{
		OperatingMode:  operatingMode,
		UseDNSServers:  useDnsServers,
		UseDomainName:  useDomainName,
		UseNTPServers:  useNTPServers,
		UseRapidCommit: useRapidCommit,
	}

	return &dhcpv6Configuration
}

// readRedfishEthernetInterfaceDHCPv6Configuration takes DHCPv6 configurations
// of the ethernet interface from the redfish client and save them into the terraform schema
func readRedfishEthernetInterfaceDHCPv6Configuration(ethernetInterface *redfish.EthernetInterface, d *schema.ResourceData) error {

	dhcpv6Config := map[string]interface{}{}

	dhcpv6Config["operating_mode"] = ethernetInterface.DHCPv6.OperatingMode
	dhcpv6Config["use_dns_servers"] = ethernetInterface.DHCPv6.UseDNSServers
	dhcpv6Config["use_domain_name"] = ethernetInterface.DHCPv6.UseDomainName
	dhcpv6Config["use_ntp_servers"] = ethernetInterface.DHCPv6.UseNTPServers
	dhcpv6Config["use_rapid_commit"] = ethernetInterface.DHCPv6.UseRapidCommit

	// Set terraform schema data
	if err := d.Set("dhcpv6", dhcpv6Config); err != nil {
		return fmt.Errorf("[CUSTOM] error setting %s: %v\n", "dhcpv6", err)
	}

	return nil
}

func updateRedfishEthernetInterfaceDHCPv6Configuration(d *schema.ResourceData) *redfish.DHCPv6Configuration {
	return createRedfishEthernetInterfaceDHCPv6Configuration(d)
}

// createRedfishEthernetInterfaceIPv4StaticAddresses turns schema values
// for ethernet interface ipv4 static addresses into a list of redfish.IPv4Address
func createRedfishEthernetInterfaceIPv4StaticAddresses(d *schema.ResourceData) []redfish.IPv4Address {

	// Get terraform schema data
	var ipv4StaticAddresses []interface{}
	if v, ok := d.GetOk("ipv4_static_addresses"); ok {
		ipv4StaticAddresses = v.([]interface{})
	}

	if len(ipv4StaticAddresses) > 1 || ipv4StaticAddresses[0] == nil {
		return nil
	}

	// Construct a list of redfish.IPv4Address for return
	var ipv4StaticAddressList []redfish.IPv4Address
	for _, v := range ipv4StaticAddresses {
		val := v.(map[string]interface{})

		var ipv4StaticAddress redfish.IPv4Address

		if v, ok := val["address"]; ok {
			ipv4StaticAddress.Address = v.(string)
		}
		if v, ok := val["address_origin"]; ok {
			ipv4StaticAddress.AddressOrigin = v.(redfish.IPv4AddressOrigin)
		}
		if v, ok := val["gateway"]; ok {
			ipv4StaticAddress.Gateway = v.(string)
		}
		if v, ok := val["subnet_mask"]; ok {
			ipv4StaticAddress.SubnetMask = v.(string)
		}

		ipv4StaticAddressList = append(ipv4StaticAddressList, ipv4StaticAddress)
	}

	return ipv4StaticAddressList
}

// readRedfishEthernetInterfaceIPv4StaticAddresses takes IPv4 static addresses
// of the ethernet interface from the redfish client and save them into the terraform schema
func readRedfishEthernetInterfaceIPv4StaticAddresses(ethernetInterface *redfish.EthernetInterface, d *schema.ResourceData) error {

	var ipv4StaticAddresses []interface{}

	for _, v := range ethernetInterface.IPv4StaticAddresses {
		var ipv4StaticAddress map[string]interface{}

		ipv4StaticAddress["address"] = v.Address
		ipv4StaticAddress["address_origin"] = v.AddressOrigin
		ipv4StaticAddress["gateway"] = v.Gateway
		ipv4StaticAddress["subnet_mask"] = v.SubnetMask

		ipv4StaticAddresses = append(ipv4StaticAddresses, ipv4StaticAddress)
	}

	// Set terraform schema data
	if err := d.Set("ipv4_static_addresses", ipv4StaticAddresses); err != nil {
		return fmt.Errorf("[CUSTOM] error setting %s: %v\n", "ipv4_static_addresses", err)
	}

	return nil
}

func updateRedfishEthernetInterfaceIPv4StaticAddresses(d *schema.ResourceData) []redfish.IPv4Address {
	return createRedfishEthernetInterfaceIPv4StaticAddresses(d)
}
