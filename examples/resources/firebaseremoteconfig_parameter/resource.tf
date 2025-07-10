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
