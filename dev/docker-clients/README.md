This package provides a convenient way to create a local Nomad cluster for
testing and development.

# Server

In order to create the cluster, first start the Nomad agent as follows from this
directory:

## Non-persistent server

```
nomad agent -dev -config docker-privileged.hcl
```

The configuration allows the Docker driver to start containers with
`Privileged` parameter.

## Persistent Server

To start a server that can be shutdown and restarted run the following:

```
nomad agent -config persistent.hcl
```

# Clients

Next, modify the count of client.nomad to run the desired number of Nomad
clients and then run the job.

```
nomad run client.nomad
```

After a few seconds, you will be able to run:

```
nomad node-status
```

And see the clients have started up.
