variable "name" {}
variable "region" {}
variable "image_id" {}
variable "instance_type" {}
variable "server_count" {}
variable "client_count" {}
variable "retry_join" {}
variable "nomad_binary" {}
variable "nic_type" {}
variable "internet_charge_type" {}
variable "internet_max_bandwidth_out" {}
variable "disk_category" {}
variable "disk_size" {}
variable "key_name" {}
variable "private_key_file" {}
variable "vpc_cidr" {}
variable "vswitch_cidr" {}
variable "zone" {}





resource "alicloud_vpc" "default" {
  name       = "tf-vpc"
  cidr_block = "${var.vpc_cidr}"
}

resource "alicloud_vswitch" "vsw" {
  vpc_id            = "${alicloud_vpc.default.id}"
  cidr_block        = "${var.vswitch_cidr}"
  availability_zone = "${var.zone}"
}

resource "alicloud_security_group" "group" {
  name        = "${var.name}"
  description = "New security group"
  vpc_id      = "${alicloud_vpc.default.id}"
}

resource "alicloud_security_group_rule" "allow_ssh" {
  type              = "ingress"
  ip_protocol       = "tcp"
  nic_type          = "${var.nic_type}"
  policy            = "accept"
  port_range        = "22/22"
  priority          = 1
  security_group_id = "${alicloud_security_group.group.id}"
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_nomad" {
  type              = "ingress"
  ip_protocol       = "tcp"
  nic_type          = "${var.nic_type}"
  policy            = "accept"
  port_range        = "4646/4646"
  priority          = 1
  security_group_id = "${alicloud_security_group.group.id}"
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_consul" {
  type              = "ingress"
  ip_protocol       = "tcp"
  nic_type          = "${var.nic_type}"
  policy            = "accept"
  port_range        = "8500/8500"
  priority          = 1
  security_group_id = "${alicloud_security_group.group.id}"
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_hdfs_namenode_ui" {
  type              = "ingress"
  ip_protocol       = "tcp"
  nic_type          = "${var.nic_type}"
  policy            = "accept"
  port_range        = "50070/50070"
  priority          = 1
  security_group_id = "${alicloud_security_group.group.id}"
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_hdfs_datanode_ui" {
  type              = "ingress"
  ip_protocol       = "tcp"
  nic_type          = "${var.nic_type}"
  policy            = "accept"
  port_range        = "50075/50075"
  priority          = 1
  security_group_id = "${alicloud_security_group.group.id}"
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_spark_history_server_ui" {
  type              = "ingress"
  ip_protocol       = "tcp"
  nic_type          = "${var.nic_type}"
  policy            = "accept"
  port_range        = "18080/18080"
  priority          = 1
  security_group_id = "${alicloud_security_group.group.id}"
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_ingress_all" {
  type              = "ingress"
  ip_protocol       = "all"
  nic_type          = "${var.nic_type}"
  policy            = "accept"
  port_range        = "-1/-1"
  priority          = 1
  security_group_id = "${alicloud_security_group.group.id}"
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_egress_all" {
  type              = "egress"
  ip_protocol       = "all"
  nic_type          = "${var.nic_type}"
  policy            = "accept"
  port_range        = "-1/-1"
  priority          = 1
  security_group_id = "${alicloud_security_group.group.id}"
  cidr_ip           = "0.0.0.0/0"
}


data "template_file" "user_data_server" {
  template = "${file("${path.root}/user-data-server.sh")}"

  vars {
    server_count = "${var.server_count}"
    region       = "${var.region}"
    retry_join   = "${var.retry_join}"
    nomad_binary = "${var.nomad_binary}"
  }
}

data "template_file" "user_data_client" {
  template = "${file("${path.root}/user-data-client.sh")}"

  vars {
    region     = "${var.region}"
    retry_join = "${var.retry_join}"
    nomad_binary = "${var.nomad_binary}"
  }
}

resource "alicloud_instance" "server" {
  image_id               = "${var.image_id}"
  instance_type          = "${var.instance_type}"
  security_groups        = ["${alicloud_security_group.group.*.id}"]
  count                  = "${var.server_count}"
  vswitch_id             = "${alicloud_vswitch.vsw.id}"

  internet_charge_type       = "${var.internet_charge_type}"
  internet_max_bandwidth_out = "${var.internet_max_bandwidth_out}"

  instance_name   = "${var.name}-server-${count.index}"
  host_name       = "${var.name}-server-${count.index}"
  key_name        = "${alicloud_key_pair.key_pair.id}"

  #Instance tags
  tags {
    Name           = "${var.name}-server-${count.index}"
    ConsulAutoJoin = "auto-join"
  }

  user_data            = "${data.template_file.user_data_server.rendered}"
}

resource "alicloud_disk" "server_disk" {
  availability_zone = "${alicloud_instance.server.0.availability_zone}"
  category          = "${var.disk_category}"
  size              = "${var.disk_size}"
  count             = "${var.server_count}"
}

resource "alicloud_disk_attachment" "server-attachment" {
  count       = "${var.server_count}"
  disk_id     = "${element(alicloud_disk.server_disk.*.id, count.index)}"
  instance_id = "${element(alicloud_instance.server.*.id, count.index)}"
}

resource "alicloud_instance" "client" {
  image_id               = "${var.image_id}"
  instance_type          = "${var.instance_type}"
  security_groups        = ["${alicloud_security_group.group.*.id}"]
  count                  = "${var.client_count}"
  vswitch_id             = "${alicloud_vswitch.vsw.id}"
  
  internet_charge_type       = "${var.internet_charge_type}"
  internet_max_bandwidth_out = "${var.internet_max_bandwidth_out}"

  instance_name   = "${var.name}-client-${count.index}"
  host_name       = "${var.name}-client-${count.index}"
  key_name        = "${alicloud_key_pair.key_pair.id}"

  depends_on             = ["alicloud_instance.server"]

  #Instance tags
  tags {
    Name           = "${var.name}-client-${count.index}"
    ConsulAutoJoin = "auto-join"
  }

  user_data            = "${data.template_file.user_data_client.rendered}"
}

resource "alicloud_disk" "client_disk" {
  availability_zone = "${alicloud_instance.client.0.availability_zone}"
  category          = "${var.disk_category}"
  size              = "${var.disk_size}"
  count             = "${var.client_count}"
}

resource "alicloud_disk_attachment" "client-attachment" {
  count       = "${var.client_count}"
  disk_id     = "${element(alicloud_disk.client_disk.*.id, count.index)}"
  instance_id = "${element(alicloud_instance.client.*.id, count.index)}"
}

resource "alicloud_ram_role" "instance_role" {
  name     = "${var.name}-role"
  services = ["ecs.aliyuncs.com"]
  force    = true
}

resource "alicloud_ram_policy" "policy" {
  name = "auto-discover-cluster"

  statement = [
    {
      effect   = "Allow"
      action   = ["ecs:*"]
      resource = ["acs:ecs:*:*:*"]
    }
  ]

  force = true
}

resource "alicloud_ram_role_policy_attachment" "role-policy" {
  policy_name = "${alicloud_ram_policy.policy.name}"
  role_name   = "${alicloud_ram_role.instance_role.name}"
  policy_type = "${alicloud_ram_policy.policy.type}"
}

resource "alicloud_ram_role_attachment" "attach" {
  role_name    = "${alicloud_ram_role.instance_role.name}"
  instance_ids = ["${alicloud_instance.server.*.id}", "${alicloud_instance.client.*.id}"]
}

resource "alicloud_key_pair" "key_pair" {
  key_name = "${var.key_name}"
  key_file = "${var.private_key_file}"
}

resource "alicloud_key_pair_attachment" "key_pair_attachment" {
  key_name     = "${alicloud_key_pair.key_pair.id}"
  instance_ids = ["${alicloud_instance.server.*.id}", "${alicloud_instance.client.*.id}"]
}

output "server_public_ips" {
  value = ["${alicloud_instance.server.*.public_ip}"]
}

output "client_public_ips" {
  value = ["${alicloud_instance.client.*.public_ip}"]
}