<powershell>
# Note this file will be used as a Terraform template_file so we
# need to escape the variable names.

# Bring ebs volume online with read-write access
Get-Disk | Where-Object IsOffline -Eq $$True | Set-Disk -IsOffline $$False
Get-Disk | Where-Object isReadOnly -Eq $$True | Set-Disk -IsReadOnly $$False

# Set Administrator password
$$admin = [adsi]("WinNT://./administrator, user")
$$admin.psbase.invoke("SetPassword", "{{admin_password}}")
</powershell>
