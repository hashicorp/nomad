datacenter = "${var.datacenter}"
client {
    enabled = true
    servers = [ ${join(",", formatlist("\"%s:4647\"", var.servers))} ]
    node_class = "linux-64bit"
}
