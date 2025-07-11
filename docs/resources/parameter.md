---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "firebaseremoteconfig_parameter Resource - firebaseremoteconfig"
subcategory: ""
description: |-
  
---

# firebaseremoteconfig_parameter (Resource)



## Example Usage

```terraform
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

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `key` (String)

### Optional

- `conditional_values` (Attributes Map) (see [below for nested schema](#nestedatt--conditional_values))
- `default_value` (Attributes) (see [below for nested schema](#nestedatt--default_value))
- `description` (String)
- `project` (String)
- `value_type` (String)

<a id="nestedatt--conditional_values"></a>
### Nested Schema for `conditional_values`

Optional:

- `use_in_app_default` (Boolean)
- `value` (String)


<a id="nestedatt--default_value"></a>
### Nested Schema for `default_value`

Optional:

- `use_in_app_default` (Boolean)
- `value` (String)
