# Terraform provider for Dalet Kowabunga KVM orchestrator

This is a Terraform provider that lets you:
- provision resources on Dalet Kowabunga instance

## Getting Started

In your `main.tf` file, specify the version you want to use:

```hcl
terraform {
  required_providers {
    kowabunga = {
      source = "dalet-oss/kowabunga"
    }
  }
}

provider "kowabunga" {
  # Configuration options
}
```

And now run terraform init:

```
$ terraform init
```

### Provider configuration

```hcl
provider "kowabunga" {
  uri      = "http://kowabunga:port"
  token    = "kowabunga_api_token"
}
```

```
## Authors

* Dalet (https://www.dalet.com/)
