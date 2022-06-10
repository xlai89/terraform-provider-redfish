terraform {
  required_providers {
    redfish = {
      version = "~> 0.2.0"
      source  = "github.com/xlai89/terraform-provider-redfish"
    }
  }
}

provider "redfish" {
  user         = var.user
  password     = var.password
  endpoint     = var.endpoint
  ssl_insecure = var.ssl_insecure
}

resource "redfish_ethernet_interface" "ethernet_interface" {
  dhcp_enabled = false
}
