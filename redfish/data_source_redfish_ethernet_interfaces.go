package redfish

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceRedfishEthernetInterfaces() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceRedfishEthernetInterfacesRead,
		Schema:      getDataSourceRedfishEthernetInterfacesSchema(),
	}
}

func getDataSourceRedfishEthernetInterfacesSchema() map[string]*schema.Schema {
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
		"ethernet_interfaces": {
			Type:        schema.TypeList,
			Description: "List of ethernet interfaces available on this instance",
			Computed:    true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"odata_id": {
						Type:        schema.TypeString,
						Description: "OData ID for the ethernet interface resource",
						Computed:    true,
					},
					"id": {
						Type:        schema.TypeString,
						Description: "Id of the ethernet interface resource",
						Computed:    true,
					},
				},
			},
		},
	}
}

func dataSourceRedfishEthernetInterfacesRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	service, err := NewConfig(m.(*schema.ResourceData), d)
	if err != nil {
		return diag.Errorf(err.Error())
	}

	//Get manager.Since this provider is thought to work with individual servers, should be only one.
	manager, err := service.Managers()
	if err != nil {
		return diag.Errorf("Error retrieving the managers: %s", err)
	}

	//Get virtual media
	ethernetInterfaceCollection, err := manager[0].EthernetInterfaces()
	if err != nil {
		return diag.Errorf("Error retrieving the virtual media instances: %s", err)
	}

	ethints := make([]map[string]interface{}, 0)
	for _, v := range ethernetInterfaceCollection {
		eiToAdd := make(map[string]interface{})
		log.Printf("Adding %s - %s", v.ODataID, v.ID)
		eiToAdd["odata_id"] = v.ODataID
		eiToAdd["id"] = v.ID
		ethints = append(ethints, eiToAdd)
	}
	d.Set("ethernet_interfaces", ethints)
	d.SetId(strconv.FormatInt(time.Now().Unix(), 10))

	return nil
}
