# terraform-provider-firebaseremoteconfig

[![CI](https://github.com/winebarrel/terraform-provider-firebaseremoteconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/winebarrel/terraform-provider-firebaseremoteconfig/actions/workflows/ci.yml)
[![terraform docs](https://img.shields.io/badge/terraform-docs-%35835CC?logo=terraform)](https://registry.terraform.io/providers/winebarrel/firebaseremoteconfig/latest/docs)

Terraform provider for Firebase Remote Config.

## Usage

```tf
terraform {
  required_providers {
    firebaseremoteconfig = {
      source = "winebarrel/firebaseremoteconfig"
    }
  }
}

provider "firebaseremoteconfig" {
  project = "my-project"
}

# import {
#   to = firebaseremoteconfig_parameter.foo
#   id = "foo"
# }

resource "firebaseremoteconfig_parameter" "foo" {
  key        = "foo"
  value_type = "JSON"

  default_value = {
    value = jsonencode({
      foo = "bar"
      zoo = 100
    })
  }
}
```

## Run locally for development

```sh
cp firebaseremoteconfig.tf.sample firebaseremoteconfig.tf
make
make tf-plan
make tf-apply
```
