# import {
#   to = firebaseremoteconfig_conditions.main
#   id = "my-project"
# }

# The order of the list matters:
# when multiple conditions evaluate to true, the earlier one wins.
resource "firebaseremoteconfig_conditions" "main" {
  conditions = [
    {
      name       = "ios"
      expression = "device.os == 'ios'"
      tag_color  = "BLUE"
    },
    {
      name       = "android"
      expression = "device.os == 'android'"
      tag_color  = "GREEN"
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
    android = {
      value = "for android"
    }
  }

  # Conditions must exist before parameters reference them.
  # To remove (or rename) a condition, remove it from conditional_values
  # and apply first, then remove the condition from the conditions list.
  depends_on = [firebaseremoteconfig_conditions.main]
}
