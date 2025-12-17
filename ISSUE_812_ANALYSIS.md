# Issue #812 Analysis: Tenant Support in available_ip_address

## Problem Statement

When using `netbox_available_ip_address` resource with both `tenant_id` and `object_type` set, NetBox API may reject the request because the tenant is not included in the initial IP allocation request.

## Root Cause

The `models.AvailableIP` struct in the upstream `github.com/fbreckle/go-netbox` library only supports the `Vrf` field:

```go
type AvailableIP struct {
    Address string `json:"address,omitempty"`
    Family  int64  `json:"family,omitempty"`
    Vrf     *NestedVRF `json:"vrf,omitempty"`
    // No Tenant field available
}
```

This means we cannot pass `Tenant` in the initial request to `/ipam/prefixes/{id}/available-ips/` even though the Terraform schema accepts `tenant_id`.

## Current Implementation

The provider handles this through a two-step process:

1. **Create**: Allocates IP from prefix/range with only VRF
2. **Update**: Immediately called after Create, sets all other attributes including tenant

See `netbox/resource_netbox_available_ip_address.go`:

- Lines 109-157: Create function
- Line 157: `return resourceNetboxAvailableIPAddressUpdate(d, m)` - Auto-update after create
- Lines 220-278: Update function sets tenant on line 233

## Why This Usually Works

The Update call happens immediately after Create in the same Terraform operation, so tenant gets set before the user sees the resource as "created".

## When This Fails

If NetBox has a validation rule that requires tenant to be set when object_type is specified, the Create may succeed but the Update could fail, OR NetBox might have changed to validate at create time.

## Solution Options

### Option 1: Update Upstream Library (Recommended)

Open an issue/PR with `github.com/fbreckle/go-netbox` to add Tenant field to AvailableIP model:

```go
type AvailableIP struct {
    Address string      `json:"address,omitempty"`
    Family  int64       `json:"family,omitempty"`
    Vrf     *NestedVRF  `json:"vrf,omitempty"`
    Tenant  *NestedTenant `json:"tenant,omitempty"`  // Add this
}
```

Then update this provider:

```go
data := models.AvailableIP{
    Vrf: &nestedvrf,
}

if tenantID != 0 {
    nestedtenant := models.NestedTenant{ID: tenantID}
    data.Tenant = &nestedtenant
}
```

### Option 2: Workaround in Terraform

Users can work around the issue by creating the IP in two steps:

```hcl
# Step 1: Create without object_type
resource "netbox_available_ip_address" "example" {
  prefix_id = data.netbox_prefix.test.id
  tenant_id = netbox_tenant.example.id
  # Don't set object_type yet
}

# Step 2: Update to add object_type
# (Run terraform apply again after first apply succeeds)
```

Or use lifecycle hooks:

```hcl
resource "netbox_available_ip_address" "example" {
  prefix_id              = data.netbox_prefix.test.id
  tenant_id              = netbox_tenant.example.id
  virtual_machine_interface_id = netbox_interface.vm.id

  lifecycle {
    ignore_changes = [virtual_machine_interface_id]
  }
}
```

### Option 3: Fork go-netbox

Fork `github.com/fbreckle/go-netbox`, add the Tenant field, and update this provider's go.mod to use your fork:

```bash
go mod edit -replace github.com/fbreckle/go-netbox=github.com/your-username/go-netbox@your-branch
```

## Testing

To verify the fix works once implemented:

```bash
# Run specific acceptance test
TEST_FUNC=TestAccNetboxAvailableIPAddress make testacc-specific-test

# Or create a test that includes tenant + object_type
```

## Status

**Current Status**: Custom fields support has been added (Issue #811). The tenant issue (Issue #812) is documented but requires upstream library changes to fully resolve.

**Recommendation**: Open an issue with the go-netbox maintainer to add Tenant field support to AvailableIP model.

## References

- Issue: <https://github.com/e-breuninger/terraform-provider-netbox/issues/812>
- NetBox API: <https://netboxlabs.com/docs/netbox/>
- go-netbox: <https://github.com/fbreckle/go-netbox>
