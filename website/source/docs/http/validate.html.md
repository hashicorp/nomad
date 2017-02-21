---
layout: "http"
page_title: "HTTP API: /v1/validate/"
sidebar_current: "docs-http-validate"
description: |-
  The '/1/validate/' endpoints are used to for validation of objects.
---

# /v1/validate/job

The `/validate/job` endpoint is to validate a Nomad job file. The local Nomad
agent forwards the request to a server. In the event a server can't be
reached the agent verifies the job file locally but skips validating driver
configurations.

## POST

<dl>
  <dt>Description</dt>
  <dd>
    Validates a Nomad job file
  </dd>

  <dt>Method</dt>
  <dd>POST</dd>

  <dt>URL</dt>
  <dd>`/v1/validate/job`</dd>

  <dt>Parameters</dt>
  <dd>
    None
  </dd>
  <dt>Body</dt>
  <dd>

  ```javascript
{
    "Job": {
        "Region": "global",
        "ID": "example",
        "ParentID": null,
        "Name": "example",
        "Type": "service",
        "Priority": 50,
        "AllAtOnce": null,
        "Datacenters": [
            "dc1"
        ],
        "Constraints": null,
        "TaskGroups": [
            {
                "Name": "cache",
                "Count": 1,
                "Constraints": null,
                "Tasks": [
                    {
                        "Name": "mongo",
                        "Driver": "exec",
                        "User": "",
                        "Config": {
                            "args": [
                                "-l",
                                "127.0.0.1",
                                "0"
                            ],
                            "command": "/bin/nc"
                        },
                        "Constraints": null,
                        "Env": null,
                        "Services": null,
                        "Resources": {
                            "CPU": 1,
                            "MemoryMB": 10,
                            "DiskMB": null,
                            "IOPS": 0,
                            "Networks": [
                                {
                                    "Public": false,
                                    "CIDR": "",
                                    "ReservedPorts": null,
                                    "DynamicPorts": [
                                        {
                                            "Label": "db111",
                                            "Value": 0
                                        },
                                        {
                                            "Label": "http111",
                                            "Value": 0
                                        }
                                    ],
                                    "IP": "",
                                    "MBits": 10
                                }
                            ]
                        },
                        "Meta": null,
                        "KillTimeout": null,
                        "LogConfig": {
                            "MaxFiles": 10,
                            "MaxFileSizeMB": 10
                        },
                        "Artifacts": null,
                        "Vault": null,
                        "Templates": null,
                        "DispatchPayload": null
                    },
                    {
                        "Name": "redis",
                        "Driver": "raw_exec",
                        "User": "",
                        "Config": {
                            "args": [
                                "-l",
                                "127.0.0.1",
                                "0"
                            ],
                            "command": "/usr/bin/nc"
                        },
                        "Constraints": null,
                        "Env": null,
                        "Services": null,
                        "Resources": {
                            "CPU": 1,
                            "MemoryMB": 10,
                            "DiskMB": null,
                            "IOPS": 0,
                            "Networks": [
                                {
                                    "Public": false,
                                    "CIDR": "",
                                    "ReservedPorts": null,
                                    "DynamicPorts": [
                                        {
                                            "Label": "db",
                                            "Value": 0
                                        },
                                        {
                                            "Label": "http",
                                            "Value": 0
                                        }
                                    ],
                                    "IP": "",
                                    "MBits": 10
                                }
                            ]
                        },
                        "Meta": null,
                        "KillTimeout": null,
                        "LogConfig": {
                            "MaxFiles": 10,
                            "MaxFileSizeMB": 10
                        },
                        "Artifacts": null,
                        "Vault": null,
                        "Templates": null,
                        "DispatchPayload": null
                    }
                ],
                "RestartPolicy": {
                    "Interval": 300000000000,
                    "Attempts": 10,
                    "Delay": 25000000000,
                    "Mode": "delay"
                },
                "EphemeralDisk": {
                    "Sticky": null,
                    "Migrate": null,
                    "SizeMB": 300
                },
                "Meta": null
            }
        ],
        "Update": {
            "Stagger": 10000000000,
            "MaxParallel": 0
        },
        "Periodic": null,
        "ParameterizedJob": null,
        "Payload": null,
        "Meta": null,
        "VaultToken": null,
        "Status": null,
        "StatusDescription": null,
        "CreateIndex": null,
        "ModifyIndex": null,
        "JobModifyIndex": null
    }
}
  ```

  </dd>


  <dt>Returns</dt>
  <dd>
    
    ```javascript
    {
        "DriverConfigValidated": true,
        "ValidationErrors": [
          "minimum CPU value is 20; got 1"
          ]
    }
    ```
  </dd>
</dl>
