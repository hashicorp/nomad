---
layout: "docs"
page_title: "Operating a Job: Task Configuration"
sidebar_current: "docs-jobops-task-config"
description: |-
  Learn how to ship task configuration in a Nomad Job.
---

# Task Configurations

Most tasks need to be paramaterized in some way. The simplest is via
command-line arguments but often times tasks consume complex or even dynamic
configurations in which the task should immediately restart and apply the new
configurations. Here we explore how to configure Nomad jobs to support many
common configuration use cases.

## Command-line Arguments

The simplest type of configuration to support is tasks which take their
configuration via command-line arguments that will not change.

Nomad has many [drivers](/docs/drivers/index.html) and most support passing
arguments to their tasks via the `args` parameter. To configure these simply
provide the appropriate arguments. Below is an example using the [`docker`
driver](/docs/drivers/docker.html) to launch memcached and set its thread count
to 4, increase log verbosity, as well as assign the correct port and address
bindings using interpolation:

```
task "memcached" {
    driver = "docker"
    
	config {
		image = "memcached:1.4.27"
		args = [
			# Set thread count
			"-t", "4",

			# Enable the highest verbosity logging mode
			"-vvv", 

			# Use interpolations to limit memory usage and bind
			# to the proper address
			"-m", "${NOMAD_MEMORY_LIMIT}",
			"-p", "${NOMAD_PORT_db}",
			"-l", "${NOMAD_ADDR_db}"
		]

		network_mode = "host"
	}

	resources {
		cpu = 500 # 500 Mhz
		memory = 256 # 256MB
		network {
			mbits = 10
			port "db" {
			}
		}
	}
}
```

In the above example, we see how easy it is to pass configuration options using
the `args` section and even see how
[interpolation](docs/jobspec/interpreted.html) allows us to pass arguments
based on the dynamic port and address Nomad chose for this task.

## Config Files

Often times applications accept their configurations using configuration files
or have so many arguments to be set it would be unwieldy to pass them via
arguments. Nomad supports downloading
[`artifacts`](/docs/jobspec/index.html#artifact_doc) prior to launching tasks.
This allows shipping of configuration files and other assets that the task
needs to run properly.

An example can be seen below, where we download two artifacts, one being the
binary to run and the other beings its configuration:

```
task "example" {
    driver = "exec"
    
	config {
		command = "my-app"
		args = ["-config", "local/config.cfg"]
	}

    # Download the binary to run
	artifact {
		source = "http://domain.com/example/my-app"
    }

	# Download the config file
	artifact {
		source = "http://domain.com/example/config.cfg"
    }
}
```

Here we can see a basic example of downloading static configuration files. By
default, an `artifact` is downloaded to the task's `local/` directory but is
[configurable](/docs/jobspec/index.html#artifact_doc).

## Dynamic Config Files

Other applications, such as load-balancers, will need to have their
configuration dynamically updated as external state changes. To support these
use cases, we can leverage
[consul-template](http://github.com/hashicorp/consul-template). To run
consul-template inside a Nomad job, we download both consul-template, the
binary we want to run and our template. In the below example we can see how to
use consul-template to update HAProxy as more webservers come up.

First we create a template file for HAProxy (please refer to consul-template documentation):

```
global
    maxconn 1000

defaults
    mode http
    timeout connect  5000
    timeout client  10000
    timeout server  10000

listen http-in
    bind {{service "my-web-lb"}} {{range service "my-web"}}
    server {{.Node}} {{.Address}}:{{.Port}}{{end}}
```

The above template will be updated to include the address of each service
registered in Consul with "my-web". As we scale the "my-web" task group in the
below job, the template should be updated and our HAProxy will load balance to
all instances.

```
job "web" {
	datacenters = ["dc1"]

	# Restrict our job to only linux as those are the binaries we are
	# downloading
	constraint {
		attribute = "${attr.kernel.name}"
		value = "linux"
	}

	group "web" {
		# Start with count 1 and scale up
		count = 1

		# Create the web server 
		task "redis" {
			driver = "exec"

			# Put our Allocation ID in an index file and start a
            # webserver to serve it. This way we know from which
            # allocation we are being served from.
			config {
				command = "/bin/bash"
				args = [
                    "-c",
                    "echo $NOMAD_ALLOC_ID > index.html; python -m SimpleHTTPServer $NOMAD_PORT_web"
                ]
			}

			resources {
				cpu = 50 
				memory = 20 
				network {
					mbits = 10
					port "web" {
					}
				}
			}

			# Add the service to register our webserver
			service {
				name = "my-web"
				port = "web"
				check {
					name = "alive"
					type = "http"
					path = "/"
					interval = "10s"
					timeout = "2s"
				}
			}
		}
	}

	# Create the loadbalancer group which will contain two tasks.
    # The first is consul-template which generates an HAProxy config and
    # the other is HAProxy itself
	group "loadbalancer" {
		# Start with count 1 and scale up
		count = 1

		# Create the web server 
		task "consul-template" {
			driver = "exec"

            # Run consul-template that takes the downloaded template and stores
            # the results in the shared alloc dir so that HAProxy can use it
			config {
				command = "consul-template"
				args = ["-template", "local/haproxy.ctmpl:alloc/haproxy.conf"]
			}

			resources {
				cpu = 500
				memory = 100
				network {
					mbits = 10
					port "inbound" {
					}
				}
			}

			# Download consul-template
			artifact {
				source = "https://releases.hashicorp.com/consul-template/0.15.0/consul-template_0.15.0_linux_amd64.zip"
			}

			# Download the template to generate.
			# Can run python -m SimpleHTTPServer to host this while testing
			artifact {
				source = "http://127.0.0.1:8000/haproxy.ctmpl"
			}
		}

        # Start HAProxy and use the config generated by consul-template    
		task "loadbalancer" {
			driver = "docker"

			config {
                # This image uses Inotify to detect changes to the config and
                # restarts HAProxy
				image = "million12/haproxy"
				network_mode = "host"
			}

			resources {
				cpu = 500
				memory = 100
			}

			env {
				# Store the path to the config
				HAPROXY_CONFIG = "alloc/haproxy.conf"
			}
		}
	}
}
```

If the above example is run, when we curl the address in which HAProxy is
listening on we see that we only receive one Allocation ID in response and the
HAProxy configuration only includes one server:

```
$ curl http://127.0.0.1:27044
da68aa6f-29db-b3d5-d8c5-fd6a2338bb13

$ curl http://127.0.0.1:27044
da68aa6f-29db-b3d5-d8c5-fd6a2338bb13

$ nomad fs 63 alloc/haproxy.conf
global
    maxconn 1000

defaults
    mode http
    timeout connect  5000
    timeout client  10000
    timeout server  10000

listen http-in
    bind 127.0.0.1:27044
    server nomad-server01 127.0.0.1:28843
```

However once we scale up the count of "my-web" from `count = 1` to `count = 3`
we see that the template was updated and we now load balance across all three
instances:

```
$ nomad fs 63 alloc/haproxy.conf
global
    maxconn 1000

defaults
    mode http
    timeout connect  5000
    timeout client  10000
    timeout server  10000

listen http-in
    bind 127.0.0.1:27044
    server nomad-server01 127.0.0.1:28843
    server nomad-server01 127.0.0.1:58402
    server nomad-server01 127.0.0.1:36143


$ curl http://127.0.0.1:27044
da68aa6f-29db-b3d5-d8c5-fd6a2338bb13

$ curl http://127.0.0.1:27044
0e83bec8-d5f6-8ae4-a2cb-99b3f0468204

$ curl http://127.0.0.1:27044
4c8a3d17-dbc8-d03a-5f77-a541eb63859d
```

While this example uses a Docker container that detects configuration changes
for simplicity, the same can be accomplished be using a PID file and having
Consul Template execute a script that restarts HAProxy using the PID file.
