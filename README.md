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

### Conditions

```tf
resource "firebaseremoteconfig_conditions" "main" {
  # The order matters: the earlier condition wins.
  conditions = [
    {
      name       = "ios"
      expression = "device.os == 'ios'"
      tag_color  = "BLUE"
    },
  ]
}

resource "firebaseremoteconfig_parameter" "bar" {
  key = "bar"

  default_value = {
    value = "default"
  }

  conditional_values = {
    ios = {
      value = "for ios"
    }
  }

  depends_on = [firebaseremoteconfig_conditions.main]
}
```

A condition referenced by parameters cannot be removed or renamed. Remove the references from `conditional_values` and apply first, then remove the condition.

## Run locally for development

```sh
cp firebaseremoteconfig.tf.sample firebaseremoteconfig.tf
make
make tf-plan
make tf-apply
```
