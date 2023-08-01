job "group_service_eto" {
  group "group" {
    service {
      name                = "example"
      enable_tag_override = true
    }
  }
}
