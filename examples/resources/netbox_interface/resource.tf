// Assume Netbox already has a VM whos name matches 'dc-west-myvm-20'
data "netbox_virtual_machine" "myvm" {
  name_regex = "dc-west-myvm-20"
}

resource "netbox_interface" "myvm_eth0" {
  name               = "eth0"
  virtual_machine_id = data.netbox_virtual_machine.myvm.id
}

// Assume existing VLAN resources 'test1' and 'test2'
resource "netbox_interface" "myvm_eth1" {
  name               = "eth1"
  enabled            = true
  mode               = "tagged"
  mtu                = 1440
  tagged_vlans       = [netbox_vlan.test1.id]
  untagged_vlan      = netbox_vlan.test2.id
  virtual_machine_id = netbox_virtual_machine.test.id
}

// To assign a MAC address, use netbox_mac_address with primary = true
resource "netbox_mac_address" "myvm_eth0_mac" {
  mac_address                  = "00:16:3E:A8:B5:D7"
  virtual_machine_interface_id = netbox_interface.myvm_eth0.id
  primary                      = true
}
