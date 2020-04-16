job "binstore-storagelocker" {
  datalog = "foo(bar, baz)."
  group "binsl" {
    datalog = "bar(bar, baz)."
    task "binstore" {
      datalog = <<EOF
zup(bar, baz).
EOF
    }
  }
}
