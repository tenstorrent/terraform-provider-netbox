resource "netbox_webhook" "test" {
  name             = "test"
  payload_url      = "https://example.com/webhook"
  body_template    = "Sample body"
  tags             = ["internal-infra"]
  secret           = "replace-me"
  ssl_verification = false
}
