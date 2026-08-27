package netbox

import (
	"strconv"
	"strings"

	"github.com/fbreckle/go-netbox/netbox/client/extras"
	"github.com/fbreckle/go-netbox/netbox/models"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var resourceNetboxWebhookHTTPMethodOptions = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

func resourceNetboxWebhook() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetboxWebhookCreate,
		Read:   resourceNetboxWebhookRead,
		Update: resourceNetboxWebhookUpdate,
		Delete: resourceNetboxWebhookDelete,

		Description: `:meta:subcategory:Extras:From the [official documentation](https://docs.netbox.dev/en/stable/integrations/webhooks/):

> A webhook is a mechanism for conveying to some external system a change that took place in NetBox. For example, you may want to notify a monitoring system whenever the status of a device is updated in NetBox. This can be done by creating a webhook for the device model in NetBox and identifying the webhook receiver. When NetBox detects a change to a device, an HTTP request containing the details of the change and who made it be sent to the specified receiver.`,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"payload_url": {
				Type:     schema.TypeString,
				Required: true,
			},
			"body_template": {
				Type:     schema.TypeString,
				Optional: true,
				DiffSuppressFunc: func(k, oldValue, newValue string, d *schema.ResourceData) bool {
					equal, _ := jsonSemanticCompare(oldValue, newValue)
					return equal
				},
				DiffSuppressOnRefresh: true,
			},
			"http_method": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice(resourceNetboxWebhookHTTPMethodOptions, false),
				Description:  buildValidValueDescription(resourceNetboxWebhookHTTPMethodOptions),
				Default:      "POST",
			},
			"http_content_type": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The complete list of official content types is available [here](https://www.iana.org/assignments/media-types/media-types.xhtml).",
				Default:     "application/json",
			},
			"additional_headers": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"ca_file_path": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"secret": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				Description: "Shared secret used by NetBox to generate the HMAC-SHA512 `X-Hook-Signature` header. " +
					"NetBox may omit or mask this value on read; Terraform keeps the configured secret in that case rather than reporting drift.",
			},
			"ssl_verification": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Enable TLS certificate verification for the payload URL. Disable with caution.",
			},
			tagsKey: tagsSchema,
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func webhookFromResourceData(d *schema.ResourceData, api *providerState) (*models.Webhook, error) {
	name := d.Get("name").(string)
	payloadURL := d.Get("payload_url").(string)
	ssl := d.Get("ssl_verification").(bool)

	tags, err := getNestedTagListFromResourceDataSet(api, d.Get(tagsAllKey))
	if err != nil {
		return nil, err
	}

	return &models.Webhook{
		Name:              &name,
		PayloadURL:        &payloadURL,
		BodyTemplate:      d.Get("body_template").(string),
		HTTPMethod:        getOptionalStr(d, "http_method", false),
		HTTPContentType:   getOptionalStr(d, "http_content_type", false),
		AdditionalHeaders: getOptionalStr(d, "additional_headers", false),
		CaFilePath:        strToPtr(getOptionalStr(d, "ca_file_path", false)),
		Secret:            webhookSecretForWrite(d),
		SslVerification:   boolToPtr(ssl),
		Tags:              tags,
	}, nil
}

func resourceNetboxWebhookCreate(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)

	data, err := webhookFromResourceData(d, api)
	if err != nil {
		return err
	}

	params := extras.NewExtrasWebhooksCreateParams().WithData(data)

	res, err := api.Extras.ExtrasWebhooksCreate(params, nil)
	if err != nil {
		return err
	}

	d.SetId(strconv.FormatInt(res.GetPayload().ID, 10))

	return resourceNetboxWebhookRead(d, m)
}

func resourceNetboxWebhookRead(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	params := extras.NewExtrasWebhooksReadParams().WithID(id)

	res, err := api.Extras.ExtrasWebhooksRead(params, nil)
	if err != nil {
		if errresp, ok := err.(*extras.ExtrasWebhooksReadDefault); ok {
			errorcode := errresp.Code()
			if errorcode == 404 {
				d.SetId("")
				return nil
			}
		}
		return err
	}

	webhook := res.GetPayload()
	d.Set("name", webhook.Name)
	d.Set("payload_url", webhook.PayloadURL)
	d.Set("body_template", webhook.BodyTemplate)
	d.Set("http_method", webhook.HTTPMethod)
	d.Set("http_content_type", webhook.HTTPContentType)
	d.Set("additional_headers", webhook.AdditionalHeaders)
	d.Set("ca_file_path", webhook.CaFilePath)

	if webhook.SslVerification != nil {
		d.Set("ssl_verification", *webhook.SslVerification)
	} else {
		d.Set("ssl_verification", true)
	}

	if webhook.Secret != nil {
		if value, ok := webhookSecretFromAPI(*webhook.Secret); ok {
			d.Set("secret", value)
		}
	}

	api.readTags(d, webhook.Tags)

	return nil
}

func resourceNetboxWebhookUpdate(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)

	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	data, err := webhookFromResourceData(d, api)
	if err != nil {
		return err
	}

	// PATCH so an omitted secret (nil / omitempty) is left untouched in NetBox.
	// PUT would treat a missing field as "reset to default" and clear it.
	params := extras.NewExtrasWebhooksPartialUpdateParams().WithID(id).WithData(data)

	_, err = api.Extras.ExtrasWebhooksPartialUpdate(params, nil)
	if err != nil {
		return err
	}

	return resourceNetboxWebhookRead(d, m)
}

func resourceNetboxWebhookDelete(d *schema.ResourceData, m interface{}) error {
	api := m.(*providerState)

	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	params := extras.NewExtrasWebhooksDeleteParams().WithID(id)

	_, err := api.Extras.ExtrasWebhooksDelete(params, nil)
	if err != nil {
		if errresp, ok := err.(*extras.ExtrasWebhooksDeleteDefault); ok {
			if errresp.Code() == 404 {
				d.SetId("")
				return nil
			}
		}
		return err
	}
	return nil
}

// webhookSecretForWrite returns the secret to send to NetBox.
// nil omits the JSON field (leave NetBox unchanged). A pointer to "" is an
// explicit clear. d.Get("secret") is "" both when unset and when set to "",
// so we inspect raw config / HasChange instead of always sending a pointer.
func webhookSecretForWrite(d *schema.ResourceData) *string {
	return webhookSecretPointer(webhookSecretConfigured(d), d.Get("secret").(string), d.HasChange("secret"))
}

func webhookSecretConfigured(d *schema.ResourceData) bool {
	cfg := d.GetRawConfig()
	if cfg.IsNull() || !cfg.IsKnown() {
		_, ok := d.GetOk("secret")
		return ok
	}
	attr := cfg.GetAttr("secret")
	return !attr.IsNull() && attr.IsKnown()
}

// webhookSecretPointer is the pure decision for tests: configured value
// (including explicit "") is always sent; a change away from a previous
// value with the attribute omitted is sent as ""; otherwise omit.
func webhookSecretPointer(configured bool, value string, changed bool) *string {
	if configured {
		return strToPtr(value)
	}
	if changed {
		empty := ""
		return &empty
	}
	return nil
}

// webhookSecretFromAPI decides whether an API-returned secret should overwrite
// Terraform state. Empty and mask-only values (e.g. "********") mean NetBox
// withheld the plaintext; returning ok=false leaves the configured secret in
// state so refresh does not produce perpetual drift or clear it.
func webhookSecretFromAPI(apiSecret string) (string, bool) {
	if apiSecret == "" {
		return "", false
	}
	if strings.Trim(apiSecret, "*") == "" {
		return "", false
	}
	return apiSecret, true
}
