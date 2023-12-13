# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "location" {}
variable "image_id" {}
variable "vm_size" {}
variable "server_count" {}
variable "client_count" {}
variable "retry_join" {}

resource "tls_private_key" "main" {
  algorithm = "RSA"
}

resource "null_resource" "main" {
  provisioner "local-exec" {
    command = "echo \"${tls_private_key.main.private_key_pem}\" > azure-hashistack.pem"
  }

  provisioner "local-exec" {
    command = "chmod 600 azure-hashistack.pem"
  }
}

resource "azurerm_resource_group" "hashistack" {
  name     = "hashistack"
  location = "${var.location}"
}

resource "azurerm_virtual_network" "hashistack-vn" {
  name                = "hashistack-vn"
  address_space       = ["10.0.0.0/16"]
  location            = "${var.location}"
  resource_group_name = "${azurerm_resource_group.hashistack.name}"
}

resource "azurerm_subnet" "hashistack-sn" {
  name                 = "hashistack-sn"
  resource_group_name  = "${azurerm_resource_group.hashistack.name}"
  virtual_network_name = "${azurerm_virtual_network.hashistack-vn.name}"
  address_prefixes     = ["10.0.2.0/24"]
}

resource "azurerm_network_security_group" "hashistack-sg" {
  name                = "hashistack-sg"
  location            = "${var.location}"
  resource_group_name = "${azurerm_resource_group.hashistack.name}"
}

resource "azurerm_network_security_rule" "hashistack-sgr-22" {
  name                        = "hashistack-sgr-22"
  resource_group_name         = "${azurerm_resource_group.hashistack.name}"
  network_security_group_name = "${azurerm_network_security_group.hashistack-sg.name}"

  priority  = 100
  direction = "Inbound"
  access    = "Allow"
  protocol  = "Tcp"

  source_address_prefix      = "*"
  source_port_range          = "*"
  destination_port_range     = "22"
  destination_address_prefix = "*"
}

resource "azurerm_network_security_rule" "hashistack-sgr-4646" {
  name                        = "hashistack-sgr-4646"
  resource_group_name         = "${azurerm_resource_group.hashistack.name}"
  network_security_group_name = "${azurerm_network_security_group.hashistack-sg.name}"

  priority  = 101
  direction = "Inbound"
  access    = "Allow"
  protocol  = "Tcp"

  source_address_prefix      = "*"
  source_port_range          = "*"
  destination_port_range     = "4646"
  destination_address_prefix = "*"
}

resource "azurerm_network_security_rule" "hashistack-sgr-8500" {
  name                        = "hashistack-sgr-8500"
  resource_group_name         = "${azurerm_resource_group.hashistack.name}"
  network_security_group_name = "${azurerm_network_security_group.hashistack-sg.name}"

  priority  = 102
  direction = "Inbound"
  access    = "Allow"
  protocol  = "Tcp"

  source_address_prefix      = "*"
  source_port_range          = "*"
  destination_port_range     = "8500"
  destination_address_prefix = "*"
}

resource "azurerm_public_ip" "hashistack-server-public-ip" {
  count               = "${var.server_count}"
  name                = "hashistack-server-ip-${count.index}"
  location            = "${var.location}"
  resource_group_name = "${azurerm_resource_group.hashistack.name}"
  allocation_method   = "Static"
}

resource "azurerm_network_interface" "hashistack-server-ni" {
  count                     = "${var.server_count}"
  name                      = "hashistack-server-ni-${count.index}"
  location                  = "${var.location}"
  resource_group_name       = "${azurerm_resource_group.hashistack.name}"
  network_security_group_id = "${azurerm_network_security_group.hashistack-sg.id}"

  ip_configuration {
    name                          = "hashistack-ipc"
    subnet_id                     = "${azurerm_subnet.hashistack-sn.id}"
    private_ip_address_allocation = "dynamic"
    public_ip_address_id          = "${element(azurerm_public_ip.hashistack-server-public-ip.*.id, count.index)}"
  }

  tags = {
    ConsulAutoJoin = "auto-join"
  }
}

resource "azurerm_virtual_machine" "server" {
  name                  = "hashistack-server-${count.index}"
  location              = "${var.location}"
  resource_group_name   = "${azurerm_resource_group.hashistack.name}"
  network_interface_ids = ["${element(azurerm_network_interface.hashistack-server-ni.*.id, count.index)}"]
  vm_size               = "${var.vm_size}"
  count                 = "${var.server_count}"

  # Uncomment this line to delete the OS disk automatically when deleting the VM
  delete_os_disk_on_termination = true

  # Uncomment this line to delete the data disks automatically when deleting the VM
  delete_data_disks_on_termination = true

  storage_image_reference {
    id = "${var.image_id}"
  }

  storage_os_disk {
    name              = "hashistack-server-osdisk-${count.index}"
    caching           = "ReadWrite"
    create_option     = "FromImage"
    managed_disk_type = "Standard_LRS"
  }

  os_profile {
    computer_name  = "hashistack-server-${count.index}"
    admin_username = "ubuntu"
    admin_password = "none"
    custom_data    = "${base64encode(data.template_file.user_data_server.rendered)}"
  }

  os_profile_linux_config {
    disable_password_authentication = true

    ssh_keys {
      path     = "/home/ubuntu/.ssh/authorized_keys"
      key_data = "${tls_private_key.main.public_key_openssh}"
    }
  }
}

data "template_file" "user_data_server" {
  template = "${file("${path.root}/user-data-server.sh")}"
  vars = {
    server_count = "${var.server_count}"
    retry_join   = "${var.retry_join}"
  }
}

resource "azurerm_public_ip" "hashistack-client-public-ip" {
  count               = "${var.client_count}"
  name                = "hashistack-client-ip-${count.index}"
  location            = "${var.location}"
  resource_group_name = "${azurerm_resource_group.hashistack.name}"
  allocation_method   = "Static"
}

resource "azurerm_network_interface" "hashistack-client-ni" {
  count                     = "${var.client_count}"
  name                      = "hashistack-client-ni-${count.index}"
  location                  = "${var.location}"
  resource_group_name       = "${azurerm_resource_group.hashistack.name}"
  network_security_group_id = "${azurerm_network_security_group.hashistack-sg.id}"

  ip_configuration {
    name                          = "hashistack-ipc"
    subnet_id                     = "${azurerm_subnet.hashistack-sn.id}"
    private_ip_address_allocation = "dynamic"
    public_ip_address_id          = "${element(azurerm_public_ip.hashistack-client-public-ip.*.id, count.index)}"
  }

  tags = {
    ConsulAutoJoin = "auto-join"
  }
}

resource "azurerm_virtual_machine" "client" {
  name                  = "hashistack-client-${count.index}"
  location              = "${var.location}"
  resource_group_name   = "${azurerm_resource_group.hashistack.name}"
  network_interface_ids = ["${element(azurerm_network_interface.hashistack-client-ni.*.id, count.index)}"]
  vm_size               = "${var.vm_size}"
  count                 = "${var.client_count}"
  depends_on            = ["azurerm_virtual_machine.server"]

  # Uncomment this line to delete the OS disk automatically when deleting the VM
  delete_os_disk_on_termination = true

  # Uncomment this line to delete the data disks automatically when deleting the VM
  delete_data_disks_on_termination = true

  storage_image_reference {
    id = "${var.image_id}"
  }

  storage_os_disk {
    name              = "hashistack-client-osdisk-${count.index}"
    caching           = "ReadWrite"
    create_option     = "FromImage"
    managed_disk_type = "Standard_LRS"
  }

  os_profile {
    computer_name  = "hashistack-client-${count.index}"
    admin_username = "ubuntu"
    admin_password = "none"
    custom_data    = "${base64encode(data.template_file.user_data_client.rendered)}"
  }

  os_profile_linux_config {
    disable_password_authentication = true

    ssh_keys {
      path     = "/home/ubuntu/.ssh/authorized_keys"
      key_data = "${tls_private_key.main.public_key_openssh}"
    }
  }
}

data "template_file" "user_data_client" {
  template = "${file("${path.root}/user-data-client.sh")}"
  vars = {
    retry_join = "${var.retry_join}"
  }
}

output "server_public_ips" {
  value = ["${azurerm_public_ip.hashistack-server-public-ip.*.ip_address}"]
}

output "client_public_ips" {
  value = ["${azurerm_public_ip.hashistack-client-public-ip.*.ip_address}"]
}
