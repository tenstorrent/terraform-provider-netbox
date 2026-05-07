package netbox

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"

	"github.com/fbreckle/go-netbox/netbox/client/ipam"
	"github.com/fbreckle/go-netbox/netbox/models"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var resourceNetboxAvailableIPAddressShuffleModeOptions = []string{"", "full", "low"}

func resourceNetboxAvailableIPAddress() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetboxAvailableIPAddressCreate,
		Read:   resourceNetboxAvailableIPAddressRead,
		Update: resourceNetboxAvailableIPAddressUpdate,
		Delete: resourceNetboxAvailableIPAddressDelete,

		Description: `:meta:subcategory:IP Address Management (IPAM):Per [the docs](https://netbox.readthedocs.io/en/stable/models/ipam/ipaddress/):

> An IP address comprises a single host address (either IPv4 or IPv6) and its subnet mask. Its mask should match exactly how the IP address is configured on an interface in the real world.
> Like a prefix, an IP address can optionally be assigned to a VRF (otherwise, it will appear in the "global" table). IP addresses are automatically arranged under parent prefixes within their respective VRFs according to the IP hierarchya.
>
> Each IP address can also be assigned an operational status and a functional role. Statuses are hard-coded in NetBox and include the following:
> * Active
> * Reserved
> * Deprecated
> * DHCP
> * SLAAC (IPv6 Stateless Address Autoconfiguration)

This resource will retrieve the next available IP address from a given prefix or IP range (specified by ID)`,

		Schema: map[string]*schema.Schema{
			"prefix_id": {
				Type:         schema.TypeInt,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"prefix_id", "ip_range_id"},
			},
			"ip_range_id": {
				Type:         schema.TypeInt,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"prefix_id", "ip_range_id"},
			},
			"ip_address": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"interface_id": {
				Type:         schema.TypeInt,
				Optional:     true,
				RequiredWith: []string{"object_type"},
			},
			"object_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(resourceNetboxIPAddressObjectTypeOptions, false),
				Description:  buildValidValueDescription(resourceNetboxIPAddressObjectTypeOptions),
				RequiredWith: []string{"interface_id"},
			},
			"virtual_machine_interface_id": {
				Type:          schema.TypeInt,
				Optional:      true,
				ConflictsWith: []string{"interface_id", "device_interface_id"},
			},
			"device_interface_id": {
				Type:          schema.TypeInt,
				Optional:      true,
				ConflictsWith: []string{"interface_id", "virtual_machine_interface_id"},
			},
			"vrf_id": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"tenant_id": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"status": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(resourceNetboxIPAddressStatusOptions, false),
				Description:  buildValidValueDescription(resourceNetboxIPAddressStatusOptions),
				Default:      "active",
			},
			"dns_name": {
				Type:     schema.TypeString,
				Optional: true,
				// NetBox always converts DNS names to lowercase
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.EqualFold(old, new)
				},
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			tagsKey: tagsSchema,
			"role": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(resourceNetboxIPAddressRoleOptions, false),
				Description:  buildValidValueDescription(resourceNetboxIPAddressRoleOptions),
			},
			customFieldsKey: customFieldsSchema,
			"shuffle_mode": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Default:      "",
				ValidateFunc: validation.StringInSlice(resourceNetboxAvailableIPAddressShuffleModeOptions, false),
				Description: "Controls IP selection strategy. Default (empty string) selects the lowest available IP. " +
					"`full` selects a random IP from all available addresses in the prefix. " +
					"`low` selects a random IP from the bottom 20% of available addresses. " +
					"Both shuffle modes skip the single lowest available IP unless it is the only one free.",
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

// pickShuffledIP selects an IP address from the available pool according to the given shuffle mode.
//
// Rules:
//   - The very lowest available IP (index 0) is excluded from the candidate pool unless it is the
//     only free IP remaining.
//   - "full": uniform random pick from the full candidate pool.
//   - "low":  uniform random pick from the first 20% of the candidate pool (at least 1 entry).
func pickShuffledIP(available []*models.AvailableIP, mode string) (string, error) {
	if len(available) == 0 {
		return "", fmt.Errorf("no available IP addresses")
	}

	candidates := available
	if len(available) > 1 {
		// skip the absolute lowest address
		candidates = available[1:]
	}

	if mode == "low" {
		// keep only the bottom 20% of the original list (after skipping the first)
		cutoff := int(math.Ceil(float64(len(candidates)) * 0.20))
		if cutoff < 1 {
			cutoff = 1
		}
		if cutoff < len(candidates) {
			candidates = candidates[:cutoff]
		}
	}

	chosen := candidates[rand.Intn(len(candidates))]
	return chosen.Address, nil
}

func resourceNetboxAvailableIPAddressCreate(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)
	prefixID := int64(d.Get("prefix_id").(int))
	rangeID := int64(d.Get("ip_range_id").(int))
	tenantID := int64(d.Get("tenant_id").(int))
	shuffleMode := d.Get("shuffle_mode").(string)

	if shuffleMode == "" {
		// Default behaviour: let NetBox pick the lowest available IP.
		data := models.WritableAvailableIP{}
		if tenantID != 0 {
			data.Tenant = tenantID
		}

		if prefixID != 0 {
			params := ipam.NewIpamPrefixesAvailableIpsCreateParams().WithID(prefixID).WithData([]*models.WritableAvailableIP{&data})
			res, err := api.Ipam.IpamPrefixesAvailableIpsCreate(params, nil)
			if err != nil {
				return err
			}
			if len(res.Payload) == 0 {
				return fmt.Errorf("no available IP addresses in prefix %d", prefixID)
			}
			d.SetId(strconv.FormatInt(res.Payload[0].ID, 10))
			d.Set("ip_address", *res.Payload[0].Address)
		}
		if rangeID != 0 {
			params := ipam.NewIpamIPRangesAvailableIpsCreateParams().WithID(rangeID).WithData([]*models.WritableAvailableIP{&data})
			res, err := api.Ipam.IpamIPRangesAvailableIpsCreate(params, nil)
			if err != nil {
				return err
			}
			if len(res.Payload) == 0 {
				return fmt.Errorf("no available IP addresses in IP range %d", rangeID)
			}
			d.SetId(strconv.FormatInt(res.Payload[0].ID, 10))
			d.Set("ip_address", *res.Payload[0].Address)
		}
		return resourceNetboxAvailableIPAddressUpdate(d, m)
	}

	// Shuffle mode: list available IPs, pick one, then create it explicitly.
	var chosenAddress string

	if prefixID != 0 {
		listParams := ipam.NewIpamPrefixesAvailableIpsListParams().WithID(prefixID)
		listRes, err := api.Ipam.IpamPrefixesAvailableIpsList(listParams, nil)
		if err != nil {
			return fmt.Errorf("error listing available IPs for prefix %d: %s", prefixID, err)
		}
		chosenAddress, err = pickShuffledIP(listRes.Payload, shuffleMode)
		if err != nil {
			return fmt.Errorf("prefix %d: %s", prefixID, err)
		}
	}
	if rangeID != 0 {
		listParams := ipam.NewIpamIPRangesAvailableIpsListParams().WithID(rangeID)
		listRes, err := api.Ipam.IpamIPRangesAvailableIpsList(listParams, nil)
		if err != nil {
			return fmt.Errorf("error listing available IPs for IP range %d: %s", rangeID, err)
		}
		var err2 error
		chosenAddress, err2 = pickShuffledIP(listRes.Payload, shuffleMode)
		if err2 != nil {
			return fmt.Errorf("IP range %d: %s", rangeID, err2)
		}
	}

	// Create the chosen IP address directly. Unlike the auto-assign endpoints
	// (IpamPrefixesAvailableIpsCreate / IpamIPRangesAvailableIpsCreate) which
	// inherit VRF from the prefix/range, IpamIPAddressesCreate requires all
	// mandatory fields explicitly. Tags must be a non-null slice. The full set
	// of remaining fields is applied by resourceNetboxAvailableIPAddressUpdate
	// immediately after.
	createData := models.WritableIPAddress{}
	createData.Address = strToPtr(chosenAddress)
	createData.Status = d.Get("status").(string)
	createData.Tags = []*models.NestedTag{}
	createData.Vrf = getOptionalInt(d, "vrf_id")
	if tenantID != 0 {
		createData.Tenant = &tenantID
	}

	createParams := ipam.NewIpamIPAddressesCreateParams().WithData(&createData)
	createRes, err := api.Ipam.IpamIPAddressesCreate(createParams, nil)
	if err != nil {
		return fmt.Errorf("error creating IP address %s: %s", chosenAddress, err)
	}

	d.SetId(strconv.FormatInt(createRes.Payload.ID, 10))
	d.Set("ip_address", *createRes.Payload.Address)

	return resourceNetboxAvailableIPAddressUpdate(d, m)
}

func resourceNetboxAvailableIPAddressRead(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	params := ipam.NewIpamIPAddressesReadParams().WithID(id)

	res, err := api.Ipam.IpamIPAddressesRead(params, nil)
	if err != nil {
		if errresp, ok := err.(*ipam.IpamIPAddressesReadDefault); ok {
			errorcode := errresp.Code()
			if errorcode == 404 {
				// If the ID is updated to blank, this tells Terraform the resource no longer exists (maybe it was destroyed out of band). Just like the destroy callback, the Read function should gracefully handle this case. https://www.terraform.io/docs/extend/writing-custom-providers.html
				d.SetId("")
				return nil
			}
		}
		return err
	}

	ipAddress := res.GetPayload()
	if ipAddress.AssignedObjectID != nil {
		vmInterfaceID := getOptionalInt(d, "virtual_machine_interface_id")
		deviceInterfaceID := getOptionalInt(d, "device_interface_id")
		interfaceID := getOptionalInt(d, "interface_id")

		switch {
		case vmInterfaceID != nil:
			d.Set("virtual_machine_interface_id", ipAddress.AssignedObjectID)
		case deviceInterfaceID != nil:
			d.Set("device_interface_id", ipAddress.AssignedObjectID)
		// if interfaceID is given, object_type must be set as well
		case interfaceID != nil:
			d.Set("object_type", ipAddress.AssignedObjectType)
			d.Set("interface_id", ipAddress.AssignedObjectID)
		}
	} else {
		d.Set("interface_id", nil)
		d.Set("object_type", "")
	}

	if ipAddress.Vrf != nil {
		d.Set("vrf_id", ipAddress.Vrf.ID)
	} else {
		d.Set("vrf_id", nil)
	}

	if ipAddress.Tenant != nil {
		d.Set("tenant_id", ipAddress.Tenant.ID)
	} else {
		d.Set("tenant_id", nil)
	}

	if ipAddress.DNSName != "" {
		d.Set("dns_name", ipAddress.DNSName)
	}

	d.Set("ip_address", ipAddress.Address)
	d.Set("description", ipAddress.Description)
	d.Set("status", ipAddress.Status.Value)
	api.readTags(d, ipAddress.Tags)
	// Always set, including with empty map, so out-of-band CF clears propagate
	// back into Terraform state as drift on the next plan.
	d.Set(customFieldsKey, getCustomFields(ipAddress.CustomFields))
	return nil
}

func resourceNetboxAvailableIPAddressUpdate(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)

	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	data := models.WritableIPAddress{}

	data.Address = strToPtr(d.Get("ip_address").(string))
	data.Status = d.Get("status").(string)

	data.Description = getOptionalStr(d, "description", false)
	data.Role = getOptionalStr(d, "role", false)
	data.DNSName = getOptionalStr(d, "dns_name", false)
	data.Vrf = getOptionalInt(d, "vrf_id")
	data.Tenant = getOptionalInt(d, "tenant_id")

	if interfaceID, ok := d.GetOk("interface_id"); ok {
		// The other possible type is dcim.interface for devices
		data.AssignedObjectType = strToPtr("virtualization.vminterface")
		data.AssignedObjectID = int64ToPtr(int64(interfaceID.(int)))
	}

	vmInterfaceID := getOptionalInt(d, "virtual_machine_interface_id")
	deviceInterfaceID := getOptionalInt(d, "device_interface_id")
	interfaceID := getOptionalInt(d, "interface_id")

	switch {
	case vmInterfaceID != nil:
		data.AssignedObjectType = strToPtr("virtualization.vminterface")
		data.AssignedObjectID = vmInterfaceID
	case deviceInterfaceID != nil:
		data.AssignedObjectType = strToPtr("dcim.interface")
		data.AssignedObjectID = deviceInterfaceID
	// if interfaceID is given, object_type must be set as well
	case interfaceID != nil:
		data.AssignedObjectType = strToPtr(d.Get("object_type").(string))
		data.AssignedObjectID = interfaceID
	// default = ip is not linked to anything
	default:
		data.AssignedObjectType = strToPtr("")
		data.AssignedObjectID = nil
	}

	var err error
	data.Tags, err = getNestedTagListFromResourceDataSet(api, d.Get(tagsAllKey))
	if err != nil {
		return err
	}

	// Send custom_fields with explicit nulls for any keys removed from config so
	// NetBox actually clears them. See customFieldsForUpdate for the full rationale.
	data.CustomFields = customFieldsForUpdate(d)

	params := ipam.NewIpamIPAddressesUpdateParams().WithID(id).WithData(&data)

	_, err = api.Ipam.IpamIPAddressesUpdate(params, nil)
	if err != nil {
		return err
	}
	return resourceNetboxAvailableIPAddressRead(d, m)
}

func resourceNetboxAvailableIPAddressDelete(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)

	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	params := ipam.NewIpamIPAddressesDeleteParams().WithID(id)

	_, err := api.Ipam.IpamIPAddressesDelete(params, nil)
	if err != nil {
		if errresp, ok := err.(*ipam.IpamIPAddressesDeleteDefault); ok {
			if errresp.Code() == 404 {
				d.SetId("")
				return nil
			}
		}
		return err
	}
	return nil
}
