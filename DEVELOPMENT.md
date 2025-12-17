# Development Guide for terraform-provider-netbox Fork

This guide documents the changes made to this fork, local development workflow, and the process for publishing your own version of the provider.

## Changes Made

### Issue #811: Custom Fields Support for available_ip_address

**What was added:**

- Added `custom_fields` attribute to the `netbox_available_ip_address` resource schema
- Updated Create, Read, and Update functions to handle custom fields
- Brings feature parity with the regular `netbox_ip_address` resource

**Files modified:**

- `netbox/resource_netbox_available_ip_address.go`
  - Line 101: Added `customFieldsKey: customFieldsSchema` to schema
  - Lines 213-216: Added custom fields reading in Read function
  - Lines 268-270: Added custom fields handling in Update function

### Issue #812: Tenant Support in available_ip_address Create

**The Issue:**
NetBox API requires tenant to be set when `object_type` is specified. The `models.AvailableIP` struct (from the upstream go-netbox library) only supports `Vrf` field in the creation request, not `Tenant`.

**Current Behavior:**
The provider already handles tenant correctly through the following flow:

1. Create call allocates the IP from the prefix/range with VRF
2. Update call (automatically invoked after Create) sets tenant, object_type, and all other attributes
3. This works for most cases

**Known Limitation:**
If you need to set both `tenant_id` AND `object_type` in the initial creation, NetBox may reject the request before the Update call completes. This is a limitation of the upstream go-netbox library's `AvailableIP` model.

**Workaround:**
If you encounter this issue:

1. Create the available IP without `object_type` first
2. Then update the resource to add `object_type` in a second apply

**Future Fix:**
This would require updating the upstream `github.com/fbreckle/go-netbox` library to add `Tenant` field to the `AvailableIP` model, or working with NetBox to relax the validation requirement.

## Local Development & Testing

### Prerequisites

- Go 1.24.0 or later
- Docker (for acceptance tests)
- Terraform CLI

### Building the Provider

Build the provider binary:

```bash
cd /Users/msollanych/git/terraform-provider-netbox
go build -o terraform-provider-netbox
```

Or install to `$GOPATH/bin`:

```bash
go install
```

### Testing Locally with Terraform

To test your local build with Terraform, use development overrides. Create or edit `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "e-breuninger/netbox" = "/Users/msollanych/git/terraform-provider-netbox"
  }

  # For all other providers, use the registry
  direct {}
}
```

Then in your Terraform configuration:

```hcl
terraform {
  required_providers {
    netbox = {
      source = "e-breuninger/netbox"
    }
  }
}

provider "netbox" {
  server_url = "http://localhost:8001"
  api_token  = "your-token"
}
```

Run `terraform plan` or `terraform apply` as normal. Terraform will use your local binary.

### Running Tests

**Unit tests:**

```bash
make test
```

**Acceptance tests** (requires Docker):

```bash
make testacc
```

This will:

1. Start a NetBox instance in Docker on port 8001
2. Run the full acceptance test suite
3. Test against the NetBox version specified in the Makefile (currently v4.4.0)

**Run a specific test:**

```bash
TEST_FUNC=TestAccNetboxAvailableIPAddress_basic make testacc-specific-test
```

**Clean up Docker volumes:**

```bash
cd docker
docker-compose down --volumes
```

### Development Workflow

1. Make your changes to the Go code
2. Run `go build` to verify it compiles
3. Test locally using development overrides
4. Run unit tests with `make test`
5. Run acceptance tests with `make testacc`
6. Commit your changes

## Publishing Your Fork

### Overview

This repository uses GoReleaser and GitHub Actions to automate releases. The setup is already configured in:

- `.goreleaser.yml` - Build and release configuration
- `.github/workflows/release.yml` - GitHub Actions workflow

### Release Process

#### 1. Set Up GPG Signing (One-time Setup)

Generate a GPG key if you don't have one:

```bash
gpg --full-generate-key
# Choose RSA, 4096 bits
# Enter your name and email
# Set a passphrase
```

Export your keys:

```bash
# Get your key ID
gpg --list-secret-keys --keyid-format=long

# Export private key (for GitHub secrets)
gpg --armor --export-secret-keys YOUR_KEY_ID

# Export public key (for Terraform Registry)
gpg --armor --export YOUR_KEY_ID > public-key.asc
```

Add secrets to your GitHub repository:

- Go to Settings → Secrets and variables → Actions
- Add `GPG_PRIVATE_KEY`: Paste your private key (including BEGIN/END lines)
- Add `PASSPHRASE`: Your GPG key passphrase

#### 2. Create a Release

```bash
# Tag a new version
git tag v1.0.0

# Push the tag to GitHub
git push origin v1.0.0
```

The GitHub Actions workflow will automatically:

1. Build binaries for all platforms (Linux, macOS, Windows × amd64, arm64)
2. Create checksums
3. Sign the checksums with your GPG key
4. Create a GitHub Release with all artifacts

#### 3. Register with Terraform Registry (Optional)

To make your provider available via Terraform Registry:

**Prerequisites:**

- GitHub repository must be public
- Repository name must match pattern: `terraform-provider-{NAME}`
- Must have at least one release with proper artifacts

**Steps:**

1. **Sign in to Terraform Registry** at <https://registry.terraform.io>

   - Use your GitHub account

2. **Publish Provider:**

   - Click "Publish" → "Provider"
   - Select your repository
   - Agree to terms

3. **Namespace Verification:**

   - Registry will ask you to verify ownership
   - Add verification file to your repo or DNS TXT record

4. **Upload GPG Public Key:**

   - Go to your user settings
   - Add your GPG public key (public-key.asc from earlier)
   - Registry uses this to verify release signatures

5. **Documentation:**
   - Registry automatically generates docs from your `docs/` directory
   - Uses `templates/` for special pages
   - Follows standard Terraform provider documentation structure

**Usage After Publishing:**

Users can then reference your provider:

```hcl
terraform {
  required_providers {
    netbox = {
      source  = "your-username/netbox"
      version = "~> 1.0"
    }
  }
}
```

### Version Management

Follow semantic versioning:

- **Major** (v2.0.0): Breaking changes
- **Minor** (v1.1.0): New features, backwards compatible
- **Patch** (v1.0.1): Bug fixes

### CI/CD Pipeline Details

#### Workflows Configured

1. **release.yml** - Triggered on `v*` tags

   - Builds multi-platform binaries
   - Signs with GPG
   - Creates GitHub release

2. **ci-testing.yml** - Triggered on push/PR to master

   - Runs unit tests
   - Runs acceptance tests against multiple NetBox versions
   - Matrix testing across NetBox v4.3.0 - v4.4.0

3. **golangci-lint.yml** - Code quality checks
4. **ensure-docs-examples.yml** - Documentation validation
5. **check-allowed-subcategories.yml** - Doc category validation

### Build Configuration

The `.goreleaser.yml` file configures:

- Target platforms: darwin, linux, windows
- Architectures: amd64, 386, arm64
- Binary naming: `terraform-provider-netbox_v{VERSION}`
- Archive format: ZIP
- Checksum algorithm: SHA256
- GPG signing for checksums

## Updating Dependencies

Update go-netbox to latest:

```bash
go get -u github.com/fbreckle/go-netbox
go mod tidy
```

Update all dependencies:

```bash
go get -u ./...
go mod tidy
```

## Troubleshooting

### "Too many open files" error on Linux

Increase file descriptor limit:

```bash
ulimit -n 2048
```

### Stale Docker volumes causing test failures

Clean up Docker:

```bash
cd docker
docker-compose down --volumes
```

### Go version mismatch

This project requires Go 1.24.0+. Update with:

```bash
brew upgrade go  # macOS
```

### Provider not found during local testing

Verify your `~/.terraformrc` has the correct path and run:

```bash
terraform init
```

## Resources

- [Terraform Provider Development](https://www.terraform.io/docs/extend/writing-custom-providers.html)
- [GoReleaser Documentation](https://goreleaser.com/)
- [Terraform Registry Publishing](https://www.terraform.io/docs/registry/providers/publishing.html)
- [NetBox API Documentation](https://netboxlabs.com/docs/netbox/)
- [go-netbox Library](https://github.com/fbreckle/go-netbox)

## Summary of Changes

This fork adds:

1. ✅ Custom fields support for `available_ip_address` resource
2. 📝 Documented tenant limitation for `available_ip_address` (upstream issue)
3. 📚 Comprehensive development and publishing documentation

The provider is now ready for local testing and can be published to your own namespace on the Terraform Registry.
