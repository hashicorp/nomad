job "foo" {
    constraint {
        attribute = "$meta.data"
        set_contains = "foo,bar,baz"
    }
}
