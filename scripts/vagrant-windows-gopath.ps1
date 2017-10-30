net use Z: "\\vmware-host\Shared Folders"
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";Z:\-go-\bin", [EnvironmentVariableTarget]::Machine)
[Environment]::SetEnvironmentVariable("GOPATH", "Z:\-go-\", [EnvironmentVariableTarget]::Machine)
