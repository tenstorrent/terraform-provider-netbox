package netbox

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"testing"

	"github.com/fbreckle/go-netbox/netbox/client/extras"
	"github.com/fbreckle/go-netbox/netbox/models"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceNetboxWebhook_schema(t *testing.T) {
	s := resourceNetboxWebhook().Schema

	tags, ok := s[tagsKey]
	if !ok {
		t.Fatal("tags schema field is missing")
	}
	if tags.Type != schema.TypeSet || !tags.Optional || tags.ForceNew {
		t.Fatalf("tags: Type=%v Optional=%v ForceNew=%v, want optional TypeSet without ForceNew", tags.Type, tags.Optional, tags.ForceNew)
	}

	secret, ok := s["secret"]
	if !ok {
		t.Fatal("secret schema field is missing")
	}
	if secret.Type != schema.TypeString || !secret.Optional || !secret.Sensitive || secret.ForceNew {
		t.Fatalf("secret: Type=%v Optional=%v Sensitive=%v ForceNew=%v, want optional sensitive string without ForceNew", secret.Type, secret.Optional, secret.Sensitive, secret.ForceNew)
	}

	ssl, ok := s["ssl_verification"]
	if !ok {
		t.Fatal("ssl_verification schema field is missing")
	}
	if ssl.Type != schema.TypeBool || !ssl.Optional || ssl.ForceNew {
		t.Fatalf("ssl_verification: Type=%v Optional=%v ForceNew=%v, want optional bool without ForceNew", ssl.Type, ssl.Optional, ssl.ForceNew)
	}
	if ssl.Default != true {
		t.Fatalf("ssl_verification Default=%#v, want true", ssl.Default)
	}

	p := Provider()
	webhook := p.ResourcesMap["netbox_webhook"]
	if _, ok := webhook.Schema[tagsAllKey]; !ok {
		t.Fatal("tags_all is missing from provider-wrapped netbox_webhook schema")
	}
}

func TestWebhookJSONIncludesSSLVerificationFalse(t *testing.T) {
	name := "kea-sync"
	url := "https://example.com/webhook"
	secret := "test-secret"
	ssl := false
	tagName := "internal-infra"
	tagSlug := "internal-infra"

	w := &models.Webhook{
		Name:            &name,
		PayloadURL:      &url,
		Secret:          &secret,
		SslVerification: &ssl,
		Tags: []*models.NestedTag{
			{Name: &tagName, Slug: &tagSlug},
		},
	}

	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	v, ok := got["ssl_verification"]
	if !ok {
		t.Fatalf("ssl_verification omitted from JSON (omitempty dropped false): %s", b)
	}
	if v != false {
		t.Fatalf("ssl_verification = %#v, want false; json=%s", v, b)
	}
	if got["secret"] != secret {
		t.Fatalf("secret missing from create payload; json=%s", b)
	}
	tags, ok := got["tags"].([]any)
	if !ok || len(tags) != 1 {
		t.Fatalf("tags missing from create payload; json=%s", b)
	}
}

func TestWebhookSecretFromAPI(t *testing.T) {
	if _, ok := webhookSecretFromAPI(""); ok {
		t.Fatal("empty API secret must not overwrite Terraform state")
	}
	if _, ok := webhookSecretFromAPI("********"); ok {
		t.Fatal("masked API secret must not overwrite Terraform state")
	}
	got, ok := webhookSecretFromAPI("plaintext-secret")
	if !ok || got != "plaintext-secret" {
		t.Fatalf("plaintext secret: got (%q, %v), want (plaintext-secret, true)", got, ok)
	}
}

func TestWebhookSecretPointer(t *testing.T) {
	if p := webhookSecretPointer(false, "", false); p != nil {
		t.Fatal("unset secret must be omitted (nil), not sent as empty string")
	}

	p := webhookSecretPointer(true, "configured", false)
	if p == nil || *p != "configured" {
		t.Fatalf("configured secret: got %v", ptrVal(p))
	}

	p = webhookSecretPointer(true, "", false)
	if p == nil || *p != "" {
		t.Fatal("explicit secret = \"\" must send empty string to clear")
	}

	p = webhookSecretPointer(false, "", true)
	if p == nil || *p != "" {
		t.Fatal("removing secret from config must send empty string to clear")
	}
}

func TestWebhookJSONOmitsNilSecret(t *testing.T) {
	name := "kea-sync"
	url := "https://example.com/webhook"
	ssl := true
	w := &models.Webhook{
		Name:            &name,
		PayloadURL:      &url,
		SslVerification: &ssl,
	}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["secret"]; ok {
		t.Fatalf("nil secret must be omitted from JSON, got %s", b)
	}
}

func ptrVal(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestAccNetboxWebhook_basic(t *testing.T) {
	testName := testAccGetTestName("webhook_basic")
	testPayloadURL := "https://example.com/webhook"
	testBodyTemplate := "Sample Body"
	testAdditionalHeaders := "Authentication: Bearer abcdef123456"
	testCaFilePath := "/etc/ssl/certs"
	resource.ParallelTest(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckNetBoxWebhookDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_webhook" "test" {
  name               = "%s"
  payload_url        = "%s"
  body_template      = "%s"
  additional_headers = "%s"
  ca_file_path = "%s"
}`, testName, testPayloadURL, testBodyTemplate, testAdditionalHeaders, testCaFilePath),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_webhook.test", "name", testName),
					resource.TestCheckResourceAttr("netbox_webhook.test", "payload_url", testPayloadURL),
					resource.TestCheckResourceAttr("netbox_webhook.test", "body_template", testBodyTemplate),
					resource.TestCheckResourceAttr("netbox_webhook.test", "additional_headers", testAdditionalHeaders),
					resource.TestCheckResourceAttr("netbox_webhook.test", "ca_file_path", testCaFilePath),
					resource.TestCheckResourceAttr("netbox_webhook.test", "ssl_verification", "true"),
				),
			},
			{
				ResourceName:            "netbox_webhook.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

func TestAccNetboxWebhook_update(t *testing.T) {
	testName := testAccGetTestName("webhook_update")
	testPayloadURL := "https://example.com/webhookupdate"
	testBodyTemplate := `{"text": "This is a sample json"}`
	testHTTPMethod := "PUT"
	testHTTPContentType := "application/xml"

	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_webhook" "test" {
	name              = "%s"
	payload_url       = "%s"
	body_template        = <<-EOT
	{"text": "This is a sample json"}
	EOT
        http_method       = "%s"
        http_content_type = "%s"
  }`, testName, testPayloadURL, testHTTPMethod, testHTTPContentType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_webhook.test", "name", testName),
					resource.TestCheckResourceAttr("netbox_webhook.test", "payload_url", testPayloadURL),
					resource.TestCheckResourceAttr("netbox_webhook.test", "body_template", testBodyTemplate),
					resource.TestCheckResourceAttr("netbox_webhook.test", "http_method", testHTTPMethod),
					resource.TestCheckResourceAttr("netbox_webhook.test", "http_content_type", testHTTPContentType),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "netbox_webhook" "test" {
  name                 = "%s_updated"
  payload_url          = "%s"
  body_template        = <<-EOT
  {"text": "This is a sample json"}
  EOT
}`, testName, testPayloadURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_webhook.test", "name", testName+"_updated"),
					resource.TestCheckResourceAttr("netbox_webhook.test", "payload_url", testPayloadURL),
					resource.TestCheckResourceAttr("netbox_webhook.test", "body_template", testBodyTemplate),
				),
			},
		},
	})
}

func TestAccNetboxWebhook_import(t *testing.T) {
	testName := testAccGetTestName("webhook_import")
	testPayloadURL := "https://test2.com/webhook"

	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_webhook" "test" {
  name                   = "%s"
  payload_url            = "%s"
}`, testName, testPayloadURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_webhook.test", "name", testName),
					resource.TestCheckResourceAttr("netbox_webhook.test", "payload_url", testPayloadURL),
				),
			},
			{
				ResourceName:            "netbox_webhook.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

func TestAccNetboxWebhook_tagsSecretSSL(t *testing.T) {
	testName := testAccGetTestName("webhook_ext")
	testPayloadURL := "https://example.com/kea-sync"
	resource.ParallelTest(t, resource.TestCase{
		Providers:    testAccProviders,
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckNetBoxWebhookDestroy,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbox_tag" "test" {
  name = "%[1]s"
}

resource "netbox_tag" "test_b" {
  name = "%[1]s-b"
}

resource "netbox_webhook" "test" {
  name              = "%[1]s"
  payload_url       = "%[2]s"
  http_method       = "POST"
  tags              = [netbox_tag.test.name]
  secret            = "initial-secret"
  ssl_verification  = false
}`, testName, testPayloadURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_webhook.test", "name", testName),
					resource.TestCheckResourceAttr("netbox_webhook.test", "payload_url", testPayloadURL),
					resource.TestCheckResourceAttr("netbox_webhook.test", "tags.#", "1"),
					resource.TestCheckResourceAttr("netbox_webhook.test", "tags.0", testName),
					resource.TestCheckResourceAttr("netbox_webhook.test", "secret", "initial-secret"),
					resource.TestCheckResourceAttr("netbox_webhook.test", "ssl_verification", "false"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "netbox_tag" "test" {
  name = "%[1]s"
}

resource "netbox_tag" "test_b" {
  name = "%[1]s-b"
}

resource "netbox_webhook" "test" {
  name              = "%[1]s"
  payload_url       = "%[2]s"
  http_method       = "POST"
  tags              = [netbox_tag.test.name]
  secret            = "initial-secret"
  ssl_verification  = false
}`, testName, testPayloadURL),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config: fmt.Sprintf(`
resource "netbox_tag" "test" {
  name = "%[1]s"
}

resource "netbox_tag" "test_b" {
  name = "%[1]s-b"
}

resource "netbox_webhook" "test" {
  name              = "%[1]s"
  payload_url       = "%[2]s"
  http_method       = "POST"
  tags              = [netbox_tag.test_b.name]
  secret            = "rotated-secret"
  ssl_verification  = true
}`, testName, testPayloadURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_webhook.test", "tags.#", "1"),
					resource.TestCheckResourceAttr("netbox_webhook.test", "tags.0", testName+"-b"),
					resource.TestCheckResourceAttr("netbox_webhook.test", "secret", "rotated-secret"),
					resource.TestCheckResourceAttr("netbox_webhook.test", "ssl_verification", "true"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "netbox_tag" "test" {
  name = "%[1]s"
}

resource "netbox_tag" "test_b" {
  name = "%[1]s-b"
}

resource "netbox_webhook" "test" {
  name             = "%[1]s"
  payload_url      = "%[2]s"
  http_method      = "POST"
  tags             = []
  secret           = ""
  ssl_verification = false
}`, testName, testPayloadURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_webhook.test", "tags.#", "0"),
					resource.TestCheckResourceAttr("netbox_webhook.test", "secret", ""),
					resource.TestCheckResourceAttr("netbox_webhook.test", "ssl_verification", "false"),
				),
			},
			{
				ResourceName:            "netbox_webhook.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
		},
	})
}

func testAccCheckNetBoxWebhookDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*providerState)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "netbox_webhook" {
			continue
		}

		// Fetch the webhook by ID
		// Retrieve our interface by referencing it's state ID for API lookup
		stateID, _ := strconv.ParseInt(rs.Primary.ID, 10, 64)
		webhook, err := client.Extras.ExtrasWebhooksRead(extras.NewExtrasWebhooksReadParams().WithID(stateID), nil)
		if err == nil && webhook != nil {
			return fmt.Errorf("Webhook %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func init() {
	resource.AddTestSweepers("netbox_webhook", &resource.Sweeper{
		Name:         "netbox_webhook",
		Dependencies: []string{},
		F: func(region string) error {
			m, err := sharedClientForRegion(region)
			if err != nil {
				return fmt.Errorf("Error getting client: %s", err)
			}
			api := m.(*providerState)
			params := extras.NewExtrasWebhooksListParams()
			res, err := api.Extras.ExtrasWebhooksList(params, nil)
			if err != nil {
				return err
			}
			for _, webhook := range res.GetPayload().Results {
				if strings.HasPrefix(*webhook.Name, testPrefix) {
					deleteParams := extras.NewExtrasWebhooksDeleteParams().WithID(webhook.ID)
					_, err := api.Extras.ExtrasWebhooksDelete(deleteParams, nil)
					if err != nil {
						return err
					}
					log.Print("[DEBUG] Deleted a webhook")
				}
			}
			return nil
		},
	})
}
