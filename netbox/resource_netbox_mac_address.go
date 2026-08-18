package netbox

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fbreckle/go-netbox/netbox/client/dcim"
	"github.com/fbreckle/go-netbox/netbox/client/virtualization"
	"github.com/fbreckle/go-netbox/netbox/models"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var resourceNetboxMACAddressObjectTypeOptions = []string{"virtualization.vminterface", "dcim.interface"}

func resourceNetboxMACAddress() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetboxMACAddressCreate,
		Read:   resourceNetboxMACAddressRead,
		Update: resourceNetboxMACAddressUpdate,
		Delete: resourceNetboxMACAddressDelete,

		Description: `:meta:subcategory:Data Center Inventory Management (DCIM):From the [official documentation](https://netboxlabs.com/docs/netbox/models/dcim/macaddress/):

> A MAC address object in NetBox comprises a single Ethernet link layer address, and represents a MAC address as reported by or assigned to a network interface. MAC addresses can be assigned to device and virtual machine interfaces. A MAC address can be specified as the primary MAC address for a given device or VM interface.`,

		Schema: map[string]*schema.Schema{
			"mac_address": {
				Type:     schema.TypeString,
				Required: true,
				// Netbox converts MAC addresses always to uppercase
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.EqualFold(old, new)
				},
			},
			"interface_id": {
				Type:         schema.TypeInt,
				Optional:     true,
				RequiredWith: []string{"object_type"},
			},
			"object_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(resourceNetboxMACAddressObjectTypeOptions, false),
				Description:  buildValidValueDescription(resourceNetboxMACAddressObjectTypeOptions),
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
			"primary": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Promote this MAC address to the primary MAC on the assigned interface. Requires the MAC to be assigned to an interface (via virtual_machine_interface_id, device_interface_id, or interface_id + object_type).",
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"comments": {
				Type:     schema.TypeString,
				Optional: true,
			},
			tagsKey:         tagsSchema,
			customFieldsKey: customFieldsSchema,
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceNetboxMACAddressCreate(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)

	data := models.MACAddress{}

	data.MacAddress = strToPtr(d.Get("mac_address").(string))

	data.Description = strToPtr(getOptionalStr(d, "description", false))
	data.Comments = strToPtr(getOptionalStr(d, "comments", false))

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
	// default = mac address is not linked to anything
	default:
		data.AssignedObjectType = strToPtr("")
		data.AssignedObjectID = nil
	}

	var err error
	data.Tags, err = getNestedTagListFromResourceDataSet(api, d.Get(tagsAllKey))
	if err != nil {
		return err
	}

	cf, ok := d.GetOk(customFieldsKey)
	if ok {
		data.CustomFields = cf
	}

	params := dcim.NewDcimMacAddressesCreateParams().WithData(&data)

	res, err := api.Dcim.DcimMacAddressesCreate(params, nil)
	if err != nil {
		return err
	}

	macID := res.GetPayload().ID
	d.SetId(strconv.FormatInt(macID, 10))

	if d.Get("primary").(bool) {
		objectType, ifaceID := macAddressResolveInterface(d)
		if objectType != "" && ifaceID != 0 {
			if err := patchInterfacePrimaryMAC(api, objectType, ifaceID, &macID); err != nil {
				return err
			}
		}
	}

	return resourceNetboxMACAddressRead(d, m)
}

func resourceNetboxMACAddressRead(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	params := dcim.NewDcimMacAddressesReadParams().WithID(id)

	res, err := api.Dcim.DcimMacAddressesRead(params, nil)

	if err != nil {
		if errresp, ok := err.(*dcim.DcimMacAddressesReadDefault); ok {
			errorcode := errresp.Code()
			if errorcode == 404 {
				// If the ID is updated to blank, this tells Terraform the resource no longer exists (maybe it was destroyed out of band). Just like the destroy callback, the Read function should gracefully handle this case. https://www.terraform.io/docs/extend/writing-custom-providers.html
				d.SetId("")
				return nil
			}
		}
		return err
	}

	macAddress := res.GetPayload()

	if macAddress.AssignedObjectID != nil && macAddress.AssignedObjectType != nil {
		vmInterfaceID := getOptionalInt(d, "virtual_machine_interface_id")
		deviceInterfaceID := getOptionalInt(d, "device_interface_id")
		interfaceID := getOptionalInt(d, "interface_id")

		switch {
		case vmInterfaceID != nil && *macAddress.AssignedObjectType == "virtualization.vminterface":
			d.Set("virtual_machine_interface_id", macAddress.AssignedObjectID)
		case deviceInterfaceID != nil && *macAddress.AssignedObjectType == "dcim.interface":
			d.Set("device_interface_id", macAddress.AssignedObjectID)
		case interfaceID != nil:
			d.Set("object_type", macAddress.AssignedObjectType)
			d.Set("interface_id", macAddress.AssignedObjectID)
		}

		primaryMACID := readInterfacePrimaryMACID(api, *macAddress.AssignedObjectType, *macAddress.AssignedObjectID)
		d.Set("primary", primaryMACID == id)
	} else {
		d.Set("interface_id", nil)
		d.Set("object_type", "")
		d.Set("primary", false)
	}

	d.Set("mac_address", macAddress.MacAddress)
	d.Set("description", macAddress.Description)
	d.Set("comments", macAddress.Comments)
	api.readTags(d, macAddress.Tags)

	cf := getCustomFields(macAddress.CustomFields)
	if cf != nil {
		d.Set(customFieldsKey, cf)
	}

	return nil
}

func resourceNetboxMACAddressUpdate(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)

	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	data := models.MACAddress{}

	data.MacAddress = strToPtr(d.Get("mac_address").(string))

	data.Description = strToPtr(getOptionalStr(d, "description", false))
	data.Comments = strToPtr(getOptionalStr(d, "comments", false))

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
	// default = mac address is not linked to anything
	default:
		data.AssignedObjectType = strToPtr("")
		data.AssignedObjectID = nil
	}

	var err error
	data.Tags, err = getNestedTagListFromResourceDataSet(api, d.Get(tagsAllKey))
	if err != nil {
		return err
	}

	cf, ok := d.GetOk(customFieldsKey)
	if ok {
		data.CustomFields = cf
	}

	params := dcim.NewDcimMacAddressesPartialUpdateParams().WithID(id).WithData(&data)

	_, err = api.Dcim.DcimMacAddressesPartialUpdate(params, nil)
	if err != nil {
		return err
	}

	oldObjType, oldIfaceID := macAddressResolveInterfaceOld(d)
	newObjType, newIfaceID := macAddressResolveInterface(d)
	interfaceChanged := oldObjType != newObjType || oldIfaceID != newIfaceID

	if d.HasChange("primary") || (interfaceChanged && d.Get("primary").(bool)) {
		if interfaceChanged && oldIfaceID != 0 {
			_ = patchInterfacePrimaryMAC(api, oldObjType, oldIfaceID, nil)
		}
		if newObjType != "" && newIfaceID != 0 {
			if d.Get("primary").(bool) {
				if err := patchInterfacePrimaryMAC(api, newObjType, newIfaceID, &id); err != nil {
					return err
				}
			} else if !interfaceChanged {
				// Only clear primary on the current interface when toggling primary off
				// on the same interface. If the interface changed, the old interface was
				// already cleared above; don't touch the new interface since this MAC
				// was never primary there.
				if err := patchInterfacePrimaryMAC(api, newObjType, newIfaceID, nil); err != nil {
					return err
				}
			}
		}
	}

	return resourceNetboxMACAddressRead(d, m)
}

func resourceNetboxMACAddressDelete(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)

	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if d.Get("primary").(bool) {
		objectType, interfaceID := macAddressResolveInterface(d)
		if objectType != "" && interfaceID != 0 {
			_ = patchInterfacePrimaryMAC(api, objectType, interfaceID, nil)
		}
	}

	params := dcim.NewDcimMacAddressesDeleteParams().WithID(id)

	_, err := api.Dcim.DcimMacAddressesDelete(params, nil)
	if err != nil {
		if errresp, ok := err.(*dcim.DcimMacAddressesDeleteDefault); ok {
			if errresp.Code() == 404 {
				d.SetId("")
				return nil
			}
		}
		return err
	}
	return nil
}

// macAddressResolveInterface determines the object_type and interface ID from
// the resource data, handling the three different assignment methods.
func macAddressResolveInterface(d *schema.ResourceData) (objectType string, interfaceID int64) {
	if v, ok := d.GetOk("virtual_machine_interface_id"); ok {
		return "virtualization.vminterface", int64(v.(int))
	}
	if v, ok := d.GetOk("device_interface_id"); ok {
		return "dcim.interface", int64(v.(int))
	}
	if v, ok := d.GetOk("interface_id"); ok {
		return d.Get("object_type").(string), int64(v.(int))
	}
	return "", 0
}

// macAddressResolveInterfaceOld returns the previous interface assignment
// using d.GetChange, for detecting reassignment during Update.
func macAddressResolveInterfaceOld(d *schema.ResourceData) (objectType string, interfaceID int64) {
	if d.HasChange("virtual_machine_interface_id") {
		old, _ := d.GetChange("virtual_machine_interface_id")
		if v, ok := old.(int); ok && v != 0 {
			return "virtualization.vminterface", int64(v)
		}
	} else if v, ok := d.GetOk("virtual_machine_interface_id"); ok {
		return "virtualization.vminterface", int64(v.(int))
	}

	if d.HasChange("device_interface_id") {
		old, _ := d.GetChange("device_interface_id")
		if v, ok := old.(int); ok && v != 0 {
			return "dcim.interface", int64(v)
		}
	} else if v, ok := d.GetOk("device_interface_id"); ok {
		return "dcim.interface", int64(v.(int))
	}

	if d.HasChange("interface_id") {
		old, _ := d.GetChange("interface_id")
		oldType, _ := d.GetChange("object_type")
		if v, ok := old.(int); ok && v != 0 {
			if t, ok := oldType.(string); ok {
				return t, int64(v)
			}
		}
	} else if v, ok := d.GetOk("interface_id"); ok {
		return d.Get("object_type").(string), int64(v.(int))
	}

	return "", 0
}

// patchInterfacePrimaryMAC PATCHes the parent interface to set or clear primary_mac_address.
// Pass macAddressID = nil to clear.
// Reads the interface first to include its existing tags in the PATCH body,
// which is required by NetBox custom validation rules that enforce tenant tags on save.
func patchInterfacePrimaryMAC(api *providerState, objectType string, interfaceID int64, macAddressID *int64) error {
	body := map[string]interface{}{
		"primary_mac_address": macAddressID,
	}

	switch objectType {
	case "virtualization.vminterface":
		readParams := virtualization.NewVirtualizationInterfacesReadParams().WithID(interfaceID)
		readRes, err := api.Virtualization.VirtualizationInterfacesRead(readParams, nil)
		if err != nil {
			return fmt.Errorf("failed to read vminterface %d before patching primary_mac_address: %w", interfaceID, err)
		}
		iface := readRes.GetPayload()
		if iface.Tags != nil {
			tags := make([]map[string]interface{}, len(iface.Tags))
			for i, t := range iface.Tags {
				tag := map[string]interface{}{}
				if t.Name != nil {
					tag["name"] = *t.Name
				}
				if t.Slug != nil {
					tag["slug"] = *t.Slug
				}
				tags[i] = tag
			}
			body["tags"] = tags
		}

		params := virtualization.NewVirtualizationInterfacesPartialUpdateParams().
			WithID(interfaceID).
			WithData(&models.WritableVMInterface{})
		_, err = api.Virtualization.VirtualizationInterfacesPartialUpdate(params, nil, macBodyOverride(body))
		if err != nil {
			return fmt.Errorf("failed to patch primary_mac_address on vminterface %d: %w", interfaceID, err)
		}
	case "dcim.interface":
		readParams := dcim.NewDcimInterfacesReadParams().WithID(interfaceID)
		readRes, err := api.Dcim.DcimInterfacesRead(readParams, nil)
		if err != nil {
			return fmt.Errorf("failed to read interface %d before patching primary_mac_address: %w", interfaceID, err)
		}
		iface := readRes.GetPayload()
		if iface.Tags != nil {
			tags := make([]map[string]interface{}, len(iface.Tags))
			for i, t := range iface.Tags {
				tag := map[string]interface{}{}
				if t.Name != nil {
					tag["name"] = *t.Name
				}
				if t.Slug != nil {
					tag["slug"] = *t.Slug
				}
				tags[i] = tag
			}
			body["tags"] = tags
		}

		params := dcim.NewDcimInterfacesPartialUpdateParams().
			WithID(interfaceID).
			WithData(&models.WritableInterface{})
		_, err = api.Dcim.DcimInterfacesPartialUpdate(params, nil, macBodyOverride(body))
		if err != nil {
			return fmt.Errorf("failed to patch primary_mac_address on interface %d: %w", interfaceID, err)
		}
	default:
		return fmt.Errorf("unsupported object_type %q for primary MAC promotion", objectType)
	}
	return nil
}

// readInterfacePrimaryMACID reads the interface and returns the ID of its
// current primary_mac_address, or 0 if none is set.
func readInterfacePrimaryMACID(api *providerState, objectType string, interfaceID int64) int64 {
	switch objectType {
	case "virtualization.vminterface":
		params := virtualization.NewVirtualizationInterfacesReadParams().WithID(interfaceID)
		res, err := api.Virtualization.VirtualizationInterfacesRead(params, nil)
		if err != nil {
			return 0
		}
		iface := res.GetPayload()
		if iface.PrimaryMacAddress != nil {
			return iface.PrimaryMacAddress.ID
		}
	case "dcim.interface":
		params := dcim.NewDcimInterfacesReadParams().WithID(interfaceID)
		res, err := api.Dcim.DcimInterfacesRead(params, nil)
		if err != nil {
			return 0
		}
		iface := res.GetPayload()
		if iface.PrimaryMacAddress != nil {
			return iface.PrimaryMacAddress.ID
		}
	}
	return 0
}

// macBodyOverrideWriter intercepts SetBodyParam to replace the serialized
// struct with a custom map, enabling minimal PATCH requests.
type macBodyOverrideWriter struct {
	runtime.ClientRequest
	body interface{}
}

func (w macBodyOverrideWriter) SetBodyParam(p interface{}) error {
	return w.ClientRequest.SetBodyParam(w.body)
}

type macBodyOverrideParams struct {
	inner runtime.ClientRequestWriter
	body  interface{}
}

func (bp macBodyOverrideParams) WriteToRequest(req runtime.ClientRequest, reg strfmt.Registry) error {
	writer := macBodyOverrideWriter{ClientRequest: req, body: bp.body}
	return bp.inner.WriteToRequest(writer, reg)
}

func macBodyOverride(body interface{}) func(*runtime.ClientOperation) {
	return func(co *runtime.ClientOperation) {
		originalParams := co.Params
		co.Params = macBodyOverrideParams{inner: originalParams, body: body}
	}
}
