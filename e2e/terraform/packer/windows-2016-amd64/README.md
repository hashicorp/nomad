# Windows Packer Build

There are a few boilerplate items in the Powershell scripts, explained below.

The default TLS protocol in the version of .NET that our Powershell cmdlets are built in it 1.0, which means plenty of properly configured HTTP servers will reject requests. The boilerplate snippet below sets this for the current script:

```
# Force TLS1.2
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
```

We need to run some of the scripts as an administrator role. The following is a safety check that we're doing so:

```
$RunningAsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
if (!$RunningAsAdmin) {
  Write-Error "Must be executed in Administrator level shell."
  exit 1
}
```
