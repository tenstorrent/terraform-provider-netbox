resource "netbox_manufacturer" "test" {
  name = "test"
}

# Minimal: just the device_type, no nested templates.
resource "netbox_device_type" "minimal" {
  model           = "minimal"
  part_number     = "123"
  manufacturer_id = netbox_manufacturer.test.id
}

# Comprehensive: a single device_type that exercises every nested template
# family. Power outlets reference power ports by name; front ports reference
# rear ports by name; inventory items can form a parent tree and optionally
# point at any other component template via component_type/component_id.
resource "netbox_device_type" "full" {
  model           = "full"
  part_number     = "456"
  manufacturer_id = netbox_manufacturer.test.id
  u_height        = 2
  is_full_depth   = true

  power_port_templates {
    name           = "psu0"
    type           = "iec-60320-c14"
    maximum_draw   = 750
    allocated_draw = 500
  }
  power_port_templates {
    name           = "psu1"
    type           = "iec-60320-c14"
    maximum_draw   = 750
    allocated_draw = 500
  }

  power_outlet_templates {
    name       = "out0"
    type       = "iec-60320-c13"
    power_port = "psu0"
    feed_leg   = "A"
  }

  interface_templates {
    name      = "mgmt0"
    type      = "1000base-t"
    mgmt_only = true
  }
  interface_templates {
    name = "eth0"
    type = "10gbase-x-sfpp"
  }

  console_port_templates {
    name = "console0"
    type = "rj-45"
  }

  console_server_port_templates {
    name = "csp0"
    type = "rj-45"
  }

  rear_port_templates {
    name      = "rp0"
    type      = "8p8c"
    positions = 4
  }

  front_port_templates {
    name               = "fp0"
    type               = "8p8c"
    rear_port          = "rp0"
    rear_port_position = 1
  }

  device_bay_templates {
    name = "bay0"
  }

  module_bay_templates {
    name     = "modbay0"
    position = "1"
  }

  inventory_item_templates {
    name = "chassis"
  }
  inventory_item_templates {
    name   = "psu-fan-a"
    parent = "chassis"
  }
}
