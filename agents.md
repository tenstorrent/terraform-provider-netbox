# Local Development for AI Agents

## Building Locally

This repository uses Terraform's `dev_overrides` feature for local development. The user's `~/.terraformrc` contains:

```hcl
provider_installation {
  dev_overrides {
    "e-breuninger/netbox" = "/Users/msollanych/git/terraform-plugin-work/terraform-provider-netbox"
  }
  direct {}
}
```

### Build Process

1. Build the provider binary in place:
```bash
cd /Users/msollanych/git/terraform-plugin-work/terraform-provider-netbox
go build -o terraform-provider-netbox
```

2. That's it. Terraform will automatically use the local binary when running `terraform plan` or `terraform apply` in any project that uses the `e-breuninger/netbox` provider.

### Important Notes

- **Do NOT** install to `~/.terraform.d/plugins/` - the dev_overrides configuration handles everything
- **Do NOT** need to run `terraform init -upgrade` after rebuilding - just rebuild and run terraform commands
- The binary name must be `terraform-provider-netbox` (not versioned)
- The binary must be in the exact directory specified in the dev_overrides path

### Testing Changes

After making code changes:
```bash
go build -o terraform-provider-netbox
cd /path/to/your/terraform/project
terraform plan  # Will use your local build
```

### Available Make Targets

See `GNUmakefile` for testing targets:
- `make test` - Run unit tests
- `make testacc` - Run acceptance tests (requires Docker)
- `make docker-up` - Start local Netbox instance for testing
- `make docker-down` - Stop local Netbox instance
- `make fmt` - Format Go code
- `make docs` - Generate documentation
