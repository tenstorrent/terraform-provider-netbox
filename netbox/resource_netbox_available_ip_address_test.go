package netbox

import (
	"fmt"
	"log"
	"regexp"
	"testing"

	"github.com/fbreckle/go-netbox/netbox/client/ipam"
	"github.com/fbreckle/go-netbox/netbox/models"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccNetboxAvailableIPAddress_basic(t *testing.T) {
	testPrefix := "1.1.2.0/24"
	testIP := "1.1.2.1/24"
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_prefix" "test" {
  prefix = "%s"
  status = "active"
  is_pool = false
}
resource "netbox_available_ip_address" "test" {
  prefix_id = netbox_prefix.test.id
  status = "active"
  dns_name = "test.mydomain.local"
  role = "loopback"
}`, testPrefix),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "ip_address", testIP),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "status", "active"),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "dns_name", "test.mydomain.local"),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "role", "loopback"),
				),
			},
		},
	})
}
func TestAccNetboxAvailableIPAddress_basic_range(t *testing.T) {
	startAddress := "1.1.5.1/24"
	endAddress := "1.1.5.50/24"
	testIP := "1.1.5.1/24"
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_ip_range" "test" {
  start_address = "%s"
  end_address = "%s"
}
resource "netbox_available_ip_address" "test_range" {
  ip_range_id = netbox_ip_range.test.id
  status = "active"
  dns_name = "test_range.mydomain.local"
}`, startAddress, endAddress),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test_range", "ip_address", testIP),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test_range", "status", "active"),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test_range", "dns_name", "test_range.mydomain.local"),
				),
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_shuffleModeUpdateDoesNotReplace(t *testing.T) {
	testPrefix := "1.1.11.0/24"
	testIP := "1.1.11.1/24"
	var initialID string
	var initialIPAddress string

	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccNetboxAvailableIPAddressShuffleModeConfig(testPrefix, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "ip_address", testIP),
					testAccCheckAvailableIPAddressStable("netbox_available_ip_address.test", &initialID, &initialIPAddress),
				),
			},
			{
				Config: testAccNetboxAvailableIPAddressShuffleModeConfig(testPrefix, "low"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "ip_address", testIP),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "shuffle_mode", "low"),
					testAccCheckAvailableIPAddressStable("netbox_available_ip_address.test", &initialID, &initialIPAddress),
				),
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_multipleIpsParallel(t *testing.T) {
	testPrefix := "1.1.3.0/24"
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_prefix" "test" {
  prefix = "%s"
  status = "active"
  is_pool = false
}
resource "netbox_available_ip_address" "test1" {
  prefix_id = netbox_prefix.test.id
  status = "active"
  dns_name = "test.mydomain.local"
}
resource "netbox_available_ip_address" "test2" {
  prefix_id = netbox_prefix.test.id
  status = "active"
  dns_name = "test.mydomain.local"
}
resource "netbox_available_ip_address" "test3" {
  prefix_id = netbox_prefix.test.id
  status = "active"
  dns_name = "test.mydomain.local"
}`, testPrefix),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("netbox_available_ip_address.test1", "ip_address"),
					resource.TestCheckResourceAttrSet("netbox_available_ip_address.test2", "ip_address"),
					resource.TestCheckResourceAttrSet("netbox_available_ip_address.test3", "ip_address"),
				),
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_multipleIpsParallel_range(t *testing.T) {
	startAddress := "1.1.6.1/24"
	endAddress := "1.1.6.50/24"
	testIP := []string{"1.1.6.1/24", "1.1.6.2/24", "1.1.6.3/24"}
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_ip_range" "test_range" {
  start_address = "%s"
  end_address = "%s"
}
resource "netbox_available_ip_address" "test_range1" {
  ip_range_id = test_range.test_range.id
  status = "active"
  dns_name = "test_range.mydomain.local"
}
resource "netbox_available_ip_address" "test_range2" {
  ip_range_id = test_range.test_range.id
  status = "active"
  dns_name = "test_range.mydomain.local"
}
resource "netbox_available_ip_address" "test_range3" {
  ip_range_id = test_range.test_range.id
  status = "active"
  dns_name = "test_range.mydomain.local"
}`, startAddress, endAddress),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test1", "ip_address", testIP[0]),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test2", "ip_address", testIP[1]),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test3", "ip_address", testIP[2]),
				),
				ExpectError: regexp.MustCompile(".*"),
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_multipleIpsSerial(t *testing.T) {
	testPrefix := "1.1.4.0/24"
	testIP := []string{"1.1.4.1/24", "1.1.4.2/24", "1.1.4.3/24"}
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_prefix" "test" {
  prefix = "%s"
  status = "active"
  is_pool = false
}
resource "netbox_available_ip_address" "test1" {
  prefix_id = netbox_prefix.test.id
  status = "active"
  dns_name = "test.mydomain.local"
}
resource "netbox_available_ip_address" "test2" {
  depends_on = [netbox_available_ip_address.test1]
  prefix_id = netbox_prefix.test.id
  status = "active"
  dns_name = "test.mydomain.local"
}
resource "netbox_available_ip_address" "test3" {
  depends_on = [netbox_available_ip_address.test2]
  prefix_id = netbox_prefix.test.id
  status = "active"
  dns_name = "test.mydomain.local"
}`, testPrefix),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test1", "ip_address", testIP[0]),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test2", "ip_address", testIP[1]),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test3", "ip_address", testIP[2]),
				),
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_multipleIpsSerial_range(t *testing.T) {
	startAddress := "1.1.7.1/24"
	endAddress := "1.1.7.50/24"
	testIP := []string{"1.1.7.1/24", "1.1.7.2/24", "1.1.7.3/24"}
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_ip_range" "test_range" {
  start_address = "%s"
  end_address = "%s"
}
resource "netbox_available_ip_address" "test_range1" {
  ip_range_id = netbox_ip_range.test_range.id
  status = "active"
  dns_name = "test_range.mydomain.local"
}
resource "netbox_available_ip_address" "test_range2" {
  depends_on = [netbox_available_ip_address.test_range1]
  ip_range_id = netbox_ip_range.test_range.id
  status = "active"
  dns_name = "test_range.mydomain.local"
}
resource "netbox_available_ip_address" "test_range3" {
  depends_on = [netbox_available_ip_address.test_range2]
  ip_range_id = netbox_ip_range.test_range.id
  status = "active"
  dns_name = "test_range.mydomain.local"
}`, startAddress, endAddress),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test_range1", "ip_address", testIP[0]),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test_range2", "ip_address", testIP[1]),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test_range3", "ip_address", testIP[2]),
				),
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_deviceByObjectType(t *testing.T) {
	startAddress := "1.2.7.1/24"
	endAddress := "1.2.7.50/24"
	testSlug := "av_ipa_dev_ot"
	testName := testAccGetTestName(testSlug)
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccNetboxIPAddressFullDeviceDependencies(testName) + fmt.Sprintf(`
resource "netbox_ip_range" "test_range" {
  start_address = "%s"
  end_address = "%s"
}
resource "netbox_available_ip_address" "test" {
  ip_range_id = netbox_ip_range.test_range.id
  status = "active"
  dns_name = "test_range.mydomain.local"
  object_type = "dcim.interface"
  interface_id = netbox_device_interface.test.id
}`, startAddress, endAddress),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "status", "active"),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "object_type", "dcim.interface"),
					resource.TestCheckResourceAttrPair("netbox_available_ip_address.test", "interface_id", "netbox_device_interface.test", "id"),
				),
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_deviceByFieldName(t *testing.T) {
	startAddress := "1.3.7.1/24"
	endAddress := "1.3.7.50/24"
	testSlug := "av_ipa_dev_fn"
	testName := testAccGetTestName(testSlug)
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccNetboxIPAddressFullDeviceDependencies(testName) + fmt.Sprintf(`
resource "netbox_ip_range" "test_range" {
  start_address = "%s"
  end_address = "%s"
}
resource "netbox_available_ip_address" "test" {
  ip_range_id = netbox_ip_range.test_range.id
  status = "active"
  dns_name = "test_range.mydomain.local"
  device_interface_id = netbox_device_interface.test.id
}`, startAddress, endAddress),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "status", "active"),
					resource.TestCheckResourceAttrPair("netbox_available_ip_address.test", "device_interface_id", "netbox_device_interface.test", "id"),
				),
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_cf(t *testing.T) {
	testPrefix := "1.1.8.0/24"
	testIP := "1.1.8.1/24"
	testSlug := "av_ipa_cf"

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_custom_field" "test" {
  name   = "%s"
  type   = "text"
  weight = 100
  content_types = ["ipam.ipaddress"]
}

resource "netbox_prefix" "test" {
  prefix = "%s"
  status = "active"
  is_pool = false
}

resource "netbox_available_ip_address" "test" {
  prefix_id = netbox_prefix.test.id
  status = "active"
  custom_fields = {
    "${netbox_custom_field.test.name}" = "test-field"
  }
}`, testSlug, testPrefix),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "ip_address", testIP),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", fmt.Sprintf("custom_fields.%s", testSlug), "test-field"),
				),
			},
			{
				ResourceName:      "netbox_available_ip_address.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"prefix_id",
				},
			},
		},
	})
}

// TestAccNetboxAvailableIPAddress_cf_clear exercises the custom_fields
// null-clearing fix: it sets a custom field, then removes the entire
// custom_fields block from config and asserts both Terraform state and
// NetBox itself drop the value, with no further drift on a subsequent plan.
func TestAccNetboxAvailableIPAddress_cf_clear(t *testing.T) {
	testPrefix := "1.1.10.0/24"
	testIP := "1.1.10.1/24"
	testSlug := "av_ipa_cf_clear"

	withCF := fmt.Sprintf(`
resource "netbox_custom_field" "test" {
  name   = "%s"
  type   = "text"
  weight = 100
  content_types = ["ipam.ipaddress"]
}

resource "netbox_prefix" "test" {
  prefix = "%s"
  status = "active"
  is_pool = false
}

resource "netbox_available_ip_address" "test" {
  prefix_id = netbox_prefix.test.id
  status = "active"
  custom_fields = {
    "${netbox_custom_field.test.name}" = "set-then-cleared"
  }
}`, testSlug, testPrefix)

	withoutCF := fmt.Sprintf(`
resource "netbox_custom_field" "test" {
  name   = "%s"
  type   = "text"
  weight = 100
  content_types = ["ipam.ipaddress"]
}

resource "netbox_prefix" "test" {
  prefix = "%s"
  status = "active"
  is_pool = false
}

resource "netbox_available_ip_address" "test" {
  prefix_id = netbox_prefix.test.id
  status = "active"
}`, testSlug, testPrefix)

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: withCF,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "ip_address", testIP),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", fmt.Sprintf("custom_fields.%s", testSlug), "set-then-cleared"),
				),
			},
			{
				Config: withoutCF,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "ip_address", testIP),
					resource.TestCheckNoResourceAttr("netbox_available_ip_address.test", fmt.Sprintf("custom_fields.%s", testSlug)),
				),
			},
			{
				Config:             withoutCF,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccNetboxAvailableIPAddress_withTenant(t *testing.T) {
	testPrefix := "1.9.1.0/24"
	testIP := "1.9.1.1/24"
	testSlug := "av_ipa_tenant"
	testName := testAccGetTestName(testSlug)
	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_tenant" "test" {
  name = "%[1]s"
  slug = "%[2]s"
}
resource "netbox_prefix" "test" {
  prefix = "%[3]s"
  status = "active"
  is_pool = false
}
resource "netbox_available_ip_address" "test" {
  prefix_id = netbox_prefix.test.id
  tenant_id = netbox_tenant.test.id
  status = "active"
  dns_name = "tenant-test.mydomain.local"
}`, testName, testSlug, testPrefix),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "ip_address", testIP),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "status", "active"),
					resource.TestCheckResourceAttr("netbox_available_ip_address.test", "dns_name", "tenant-test.mydomain.local"),
					resource.TestCheckResourceAttrPair("netbox_available_ip_address.test", "tenant_id", "netbox_tenant.test", "id"),
				),
			},
		},
	})
}

func testAccNetboxAvailableIPAddressShuffleModeConfig(prefix string, shuffleMode string) string {
	shuffleModeLine := ""
	if shuffleMode != "" {
		shuffleModeLine = fmt.Sprintf("  shuffle_mode = %q\n", shuffleMode)
	}

	return fmt.Sprintf(`
resource "netbox_prefix" "test" {
  prefix = "%[1]s"
  status = "active"
  is_pool = false
}
resource "netbox_available_ip_address" "test" {
  prefix_id = netbox_prefix.test.id
  status = "active"
  dns_name = "shuffle-mode-update.mydomain.local"
%[2]s}`, prefix, shuffleModeLine)
}

func testAccCheckAvailableIPAddressStable(resourceName string, initialID *string, initialIPAddress *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if rs.Primary == nil {
			return fmt.Errorf("resource %s has no primary instance", resourceName)
		}

		ipAddress := rs.Primary.Attributes["ip_address"]
		if *initialID == "" {
			*initialID = rs.Primary.ID
			*initialIPAddress = ipAddress
			return nil
		}

		if rs.Primary.ID != *initialID {
			return fmt.Errorf("expected %s ID to remain %s, got %s", resourceName, *initialID, rs.Primary.ID)
		}
		if ipAddress != *initialIPAddress {
			return fmt.Errorf("expected %s ip_address to remain %s, got %s", resourceName, *initialIPAddress, ipAddress)
		}
		return nil
	}
}

func init() {
	resource.AddTestSweepers("netbox_available_ip_address", &resource.Sweeper{
		Name:         "netbox_available_ip_address",
		Dependencies: []string{},
		F: func(region string) error {
			m, err := sharedClientForRegion(region)
			if err != nil {
				return fmt.Errorf("Error getting client: %s", err)
			}
			api := m.(*providerState)
			params := ipam.NewIpamIPAddressesListParams()
			res, err := api.Ipam.IpamIPAddressesList(params, nil)
			if err != nil {
				return err
			}
			for _, ipAddress := range res.GetPayload().Results {
				if len(ipAddress.Tags) > 0 && (ipAddress.Tags[0] == &models.NestedTag{Name: strToPtr("acctest"), Slug: strToPtr("acctest")}) {
					deleteParams := ipam.NewIpamIPAddressesDeleteParams().WithID(ipAddress.ID)
					_, err := api.Ipam.IpamIPAddressesDelete(deleteParams, nil)
					if err != nil {
						return err
					}
					log.Print("[DEBUG] Deleted an ip address")
				}
			}
			return nil
		},
	})
}

// --- Unit tests for pickShuffledIP (no NetBox required) ---

func makeAvailableIPs(addresses []string) []*models.AvailableIP {
	ips := make([]*models.AvailableIP, len(addresses))
	for i, a := range addresses {
		addr := a
		ips[i] = &models.AvailableIP{Address: addr}
	}
	return ips
}

func TestPickShuffledIP_emptyPool(t *testing.T) {
	_, err := pickShuffledIP([]*models.AvailableIP{}, "full")
	if err == nil {
		t.Fatal("expected error for empty pool, got nil")
	}
}

func TestResourceNetboxAvailableIPAddress_shuffleModeDoesNotForceNew(t *testing.T) {
	field, ok := resourceNetboxAvailableIPAddress().Schema["shuffle_mode"]
	if !ok {
		t.Fatal("shuffle_mode schema field is missing")
	}
	if field.ForceNew {
		t.Fatal("shuffle_mode must not force replacement of an already allocated IP")
	}
}

func TestPickShuffledIP_singleEntry_usesIt(t *testing.T) {
	pool := makeAvailableIPs([]string{"10.0.0.1/24"})
	got, err := pickShuffledIP(pool, "full")
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.0.0.1/24" {
		t.Errorf("expected 10.0.0.1/24, got %s", got)
	}
}

func TestPickShuffledIP_full_neverPicksFirst(t *testing.T) {
	addrs := []string{
		"10.0.0.1/24", "10.0.0.2/24", "10.0.0.3/24",
		"10.0.0.4/24", "10.0.0.5/24", "10.0.0.6/24",
	}
	pool := makeAvailableIPs(addrs)
	for i := 0; i < 200; i++ {
		got, err := pickShuffledIP(pool, "full")
		if err != nil {
			t.Fatal(err)
		}
		if got == "10.0.0.1/24" {
			t.Errorf("full shuffle picked the first IP (10.0.0.1/24) on iteration %d", i)
		}
	}
}

func TestPickShuffledIP_full_coversAllNonFirst(t *testing.T) {
	addrs := []string{
		"10.0.0.1/24", "10.0.0.2/24", "10.0.0.3/24",
		"10.0.0.4/24", "10.0.0.5/24",
	}
	pool := makeAvailableIPs(addrs)
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		got, err := pickShuffledIP(pool, "full")
		if err != nil {
			t.Fatalf("pickShuffledIP returned unexpected error: %v", err)
		}
		seen[got] = true
	}
	// Every address except the first should appear
	for _, a := range addrs[1:] {
		if !seen[a] {
			t.Errorf("full shuffle never picked %s in 2000 iterations", a)
		}
	}
	if seen[addrs[0]] {
		t.Errorf("full shuffle picked the first IP %s", addrs[0])
	}
}

func TestPickShuffledIP_low_staysInBottom20Percent(t *testing.T) {
	// 10 IPs: pool[0] excluded, candidates = pool[1:9] (8 entries)
	// 20% of 8 = 1.6 -> ceil = 2, so only pool[1] and pool[2] should be picked
	addrs := make([]string, 10)
	for i := range addrs {
		addrs[i] = fmt.Sprintf("10.0.0.%d/24", i+1)
	}
	pool := makeAvailableIPs(addrs)
	allowed := map[string]bool{addrs[1]: true, addrs[2]: true}
	for i := 0; i < 500; i++ {
		got, err := pickShuffledIP(pool, "low")
		if err != nil {
			t.Fatal(err)
		}
		if !allowed[got] {
			t.Errorf("low shuffle picked %s which is outside allowed set %v", got, allowed)
		}
	}
}

func TestPickShuffledIP_low_twoEntries_picksSecond(t *testing.T) {
	pool := makeAvailableIPs([]string{"10.0.0.1/24", "10.0.0.2/24"})
	for i := 0; i < 50; i++ {
		got, err := pickShuffledIP(pool, "low")
		if err != nil {
			t.Fatal(err)
		}
		if got != "10.0.0.2/24" {
			t.Errorf("expected 10.0.0.2/24, got %s", got)
		}
	}
}

func TestPickShuffledIP_low_singleEntry_usesIt(t *testing.T) {
	pool := makeAvailableIPs([]string{"10.0.0.1/24"})
	got, err := pickShuffledIP(pool, "low")
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.0.0.1/24" {
		t.Errorf("expected 10.0.0.1/24, got %s", got)
	}
}
