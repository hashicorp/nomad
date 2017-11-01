variable "location" {}
variable "image_id" {}
variable "vm_size" {}
variable "server_count" {}
variable "client_count" {}
variable "retry_join" {}

resource "azurerm_resource_group" "nomad-rg" {
  name     = "nomad-rg"
  location = "East US"
}

resource "azurerm_virtual_network" "nomad-vn" {
  name                = "nomad-vn"
  address_space       = ["10.0.0.0/16"]
  location            = "East US"
  resource_group_name = "${azurerm_resource_group.nomad-rg.name}"
}

resource "azurerm_subnet" "nomad-sn" {
  name                 = "nomad-sn"
  resource_group_name  = "${azurerm_resource_group.nomad-rg.name}"
  virtual_network_name = "${azurerm_virtual_network.nomad-vn.name}"
  address_prefix       = "10.0.2.0/24"
}

resource "azurerm_network_interface" "nomad-ni" {
  name                = "nomad-ni"
  location            = "East US"
  resource_group_name = "${azurerm_resource_group.nomad-rg.name}"

  ip_configuration {
    name                          = "nomad-ipc"
    subnet_id                     = "${azurerm_subnet.nomad-sn.id}"
    private_ip_address_allocation = "dynamic"
  }
}

resource "azurerm_managed_disk" "test" {
  name                 = "datadisk_existing"
  location             = "East US"
  resource_group_name  = "${azurerm_resource_group.nomad-rg.name}"
  storage_account_type = "Standard_LRS"
  create_option        = "Empty"
  disk_size_gb         = "1023"
}

resource "tls_private_key" "main" {
  algorithm = "RSA"
}

resource "null_resource" "main" {
  provisioner "local-exec" {
    command = "echo \"${tls_private_key.main.private_key_pem}\" > azure-nomad.pem"
  }

  provisioner "local-exec" {
    command = "chmod 600 azure-nomad.pem"
  }
}

resource "azurerm_virtual_machine" "server" {
  name                  = "hashistack-server-${count.index}"
  location              = "East US"
  resource_group_name   = "${azurerm_resource_group.nomad-rg.name}"
  network_interface_ids = ["${azurerm_network_interface.nomad-ni.id}"]
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
    name              = "nomad-osdisk1"
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

  vars {
    server_count      = "${var.server_count}"
    location          = "${var.location}"
    retry_join        = "${var.retry_join}"
  }
}
