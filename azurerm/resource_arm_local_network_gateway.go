package azurerm

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmLocalNetworkGateway() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmLocalNetworkGatewayCreate,
		Read:   resourceArmLocalNetworkGatewayRead,
		Update: resourceArmLocalNetworkGatewayCreate,
		Delete: resourceArmLocalNetworkGatewayDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"location": locationSchema(),

			"resource_group_name": resourceGroupNameSchema(),

			"gateway_address": {
				Type:     schema.TypeString,
				Required: true,
			},

			"address_space": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"bgp_settings": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"asn": {
							Type:     schema.TypeInt,
							Required: true,
						},

						"bgp_peering_address": {
							Type:     schema.TypeString,
							Required: true,
						},

						"peer_weight": {
							Type:     schema.TypeInt,
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func resourceArmLocalNetworkGatewayCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).localNetConnClient

	name := d.Get("name").(string)
	location := d.Get("location").(string)
	resGroup := d.Get("resource_group_name").(string)
	ipAddress := d.Get("gateway_address").(string)

	addressSpaces := expandLocalNetworkGatewayAddressSpaces(d)

	bgpSettings, err := expandLocalNetworkGatewayBGPSettings(d)
	if err != nil {
		return err
	}

	gateway := network.LocalNetworkGateway{
		Name:     &name,
		Location: &location,
		LocalNetworkGatewayPropertiesFormat: &network.LocalNetworkGatewayPropertiesFormat{
			LocalNetworkAddressSpace: &network.AddressSpace{
				AddressPrefixes: &addressSpaces,
			},
			GatewayIPAddress: &ipAddress,
			BgpSettings:      bgpSettings,
		},
	}

	_, createError := client.CreateOrUpdate(resGroup, name, gateway, make(chan struct{}))
	err = <-createError
	if err != nil {
		return fmt.Errorf("Error creating Local Network Gateway %q: %+v", name, err)
	}

	read, err := client.Get(resGroup, name)
	if err != nil {
		return err
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read Local Network Gateway ID %q (resource group %q) ID", name, resGroup)
	}

	d.SetId(*read.ID)

	return resourceArmLocalNetworkGatewayRead(d, meta)
}

func resourceArmLocalNetworkGatewayRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).localNetConnClient

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	name := id.Path["localNetworkGateways"]
	resGroup := id.ResourceGroup

	resp, err := client.Get(resGroup, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error reading the state of Local Network Gateway %q (Resource Group %q): %+v", name, resGroup, err)
	}

	d.Set("name", resp.Name)
	d.Set("resource_group_name", resGroup)
	d.Set("location", azureRMNormalizeLocation(*resp.Location))

	if props := resp.LocalNetworkGatewayPropertiesFormat; props != nil {
		d.Set("gateway_address", props.GatewayIPAddress)

		if lnas := props.LocalNetworkAddressSpace; lnas != nil {
			if prefixes := lnas.AddressPrefixes; prefixes != nil {
				d.Set("address_space", *prefixes)
			}
		}
		flattenedSettings := flattenLocalNetworkGatewayBGPSettings(props.BgpSettings)
		if err := d.Set("bgp_settings", flattenedSettings); err != nil {
			return err
		}
	}

	return nil
}

func resourceArmLocalNetworkGatewayDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).localNetConnClient

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	name := id.Path["localNetworkGateways"]
	resGroup := id.ResourceGroup

	deleteResp, error := client.Delete(resGroup, name, make(chan struct{}))
	resp := <-deleteResp
	err = <-error

	if err != nil {
		if utils.ResponseWasNotFound(resp) {
			return nil
		}

		return fmt.Errorf("Error issuing delete request for local network gateway %q: %+v", name, err)
	}

	return nil
}

func expandLocalNetworkGatewayBGPSettings(d *schema.ResourceData) (*network.BgpSettings, error) {
	v, exists := d.GetOk("bgp_settings")
	if !exists {
		return nil, nil
	}

	settings := v.([]interface{})
	setting := settings[0].(map[string]interface{})

	bgpSettings := network.BgpSettings{
		Asn:               utils.Int64(int64(setting["asn"].(int))),
		BgpPeeringAddress: utils.String(setting["bgp_peering_address"].(string)),
		PeerWeight:        utils.Int32(int32(setting["peer_weight"].(int))),
	}

	return &bgpSettings, nil
}

func expandLocalNetworkGatewayAddressSpaces(d *schema.ResourceData) []string {
	prefixes := make([]string, 0)

	for _, pref := range d.Get("address_space").([]interface{}) {
		prefixes = append(prefixes, pref.(string))
	}

	return prefixes
}

func flattenLocalNetworkGatewayBGPSettings(input *network.BgpSettings) []interface{} {
	output := make(map[string]interface{}, 0)

	if input == nil {
		return []interface{}{}
	}

	output["asn"] = int(*input.Asn)
	output["bgp_peering_address"] = *input.BgpPeeringAddress
	output["peer_weight"] = int(*input.PeerWeight)

	return []interface{}{output}
}
