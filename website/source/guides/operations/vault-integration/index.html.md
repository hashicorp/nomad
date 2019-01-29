---
layout: "guides"
page_title: "Vault Integration and Retrieving Dynamic Secrets"
sidebar_current: "guides-operations-vault-integration"
description: |-
  Learn how to deploy an application in Nomad and retrieve dynamic credentials
  by integrating with Vault.
---

# Vault Integration and Retrieving Dynamic Secrets

Nomad integrates seamlessly with [Vault][vault] and allows your application to
retrieve dynamic credentials for various tasks. In this guide, you will deploy a
web application that needs to authenticate against [PostgreSQL][postgresql] to
display data from a table to the user.

## Reference Material

- [Vault Integration Docs Page][vault-integration]
- [Nomad Template Stanza Integration with Vault][nomad-template-vault]
- [Secrets Task Directory][secrets-task-directory]

## Estimated Time to Complete

20 minutes

## Challenge

Think of a scenario where a Nomad operator needs to deploy an application that
can quickly and safely retrieve dynamic credentials to authenticate against a
database and return information.

## Solution

Deploy Vault and configure the nodes in your Nomad cluster to integrate with it.
Use the appropriate [templating syntax][nomad-template-vault] to retrieve
credentials from Vault and then store those credentials in the
[secrets][secrets-task-directory] task directory to be consumed by the Nomad task.

## Prerequisites

To perform the tasks described in this guide, you need to have a Nomad
environment with Consul and Vault installed. You can use this [repo][repo]
to easily provision a sandbox environment. This guide will assume a cluster with
one server node and three client nodes.

-> **Please Note:** This guide is for demo purposes and is only using a single
Nomad server with Vault installed alongside. In a production cluster, 3 or 5 Nomad server nodes are recommended along with a separate Vault cluster.

## Steps

### Step 1: Initialize Vault Server

Run the following command to initialize Vault server and receive an
[unseal][seal] key and initial root [token][token]. Be sure to note the unseal
key and initial root token as you will need these two pieces of information.

```shell
$ vault operator init -key-shares=1 -key-threshold=1
```

The `vault operator init` command above creates a single Vault unseal key for
convenience. For a production environment, it is recommended that you create at
least five unseal key shares and securely distribute them to independent
operators. The `vault operator init` command defaults to five key shares and a key threshold of three. If you provisioned more than one server, the others will become standby nodes but should still be unsealed.

### Step 2: Unseal Vault

Run the following command and then provide your unseal key to Vault.

```shell
$ vault operator unseal
```
The output of unsealing Vault will look similar to the following:

```shell
Key                    Value
---                    -----
Seal Type              shamir
Initialized            true
Sealed                 false
Total Shares           1
Threshold              1
Version                0.11.4
Cluster Name           vault-cluster-d12535e5
Cluster ID             49383931-c782-fdc6-443e-7681e7b15aca
HA Enabled             true
HA Cluster             n/a
HA Mode                standby
Active Node Address    <none>
```

### Step 3: Log in to Vault

Use the [login][login] command to authenticate yourself against Vault using the
initial root token you received earlier. You will need to authenticate to run
the necessary commands to write policies, create roles, and configure a
connection to your database.

```shell
$ vault login <your initial root token>
```
If your login is successful, you will see output similar to what is shown below:

```shell
Success! You are now authenticated. The token information displayed below
is already stored in the token helper. You do NOT need to run "vault login"
again. Future Vault requests will automatically use this token.
...
```
### Step 4: Write the Policy for the Nomad Server Token

To use the Vault integration, you must provide a Vault token to your Nomad
servers. Although you can provide your root token to easily get started, the
recommended approach is to use a token [role][role] based token.
This first requires writing a policy that you will attach to the token you
provide to your Nomad servers. By using this approach, you can limit the set of [policies][policy] that tasks managed by Nomad can access.

For this exercise, use the following policy for the token you will create for your Nomad server. Place this policy in a file named `nomad-server-policy.hcl`.

```'hcl
# Allow creating tokens under "nomad-cluster" token role. The token role name
# should be updated if "nomad-cluster" is not used.
path "auth/token/create/nomad-cluster" {
  capabilities = ["update"]
}

# Allow looking up "nomad-cluster" token role. The token role name should be
# updated if "nomad-cluster" is not used.
path "auth/token/roles/nomad-cluster" {
  capabilities = ["read"]
}

# Allow looking up the token passed to Nomad to validate # the token has the
# proper capabilities. This is provided by the "default" policy.
path "auth/token/lookup-self" {
  capabilities = ["read"]
}

# Allow looking up incoming tokens to validate they have permissions to access
# the tokens they are requesting. This is only required if
# `allow_unauthenticated` is set to false.
path "auth/token/lookup" {
  capabilities = ["update"]
}

# Allow revoking tokens that should no longer exist. This allows revoking
# tokens for dead tasks.
path "auth/token/revoke-accessor" {
  capabilities = ["update"]
}

# Allow checking the capabilities of our own token. This is used to validate the
# token upon startup.
path "sys/capabilities-self" {
  capabilities = ["update"]
}

# Allow our own token to be renewed.
path "auth/token/renew-self" {
  capabilities = ["update"]
}
```
You can now write a policy called `nomad-server` by running the following command:

```shell
$ vault policy write nomad-server nomad-server-policy.hcl
```
You should see the following output:

```shell
Success! Uploaded policy: nomad-server
```
You will generate the actual token in the next few steps.

### Step 5: Create a Token Role

At this point, you must create a Vault token role that Nomad can use. The token
role allows you to limit what Vault policies are are accessible by jobs
submitted to Nomad. We will use the following token role:

```json
{
  "allowed_policies": "access-tables",
  "explicit_max_ttl": 0,
  "name": "nomad-cluster",
  "orphan": true,
  "period": 259200,
  "renewable": true
}
```
Please notice that the `access-tables` policy is listed under the `allowed_policies` key. We have not created this policy yet, but it will be used by our job to
retrieve credentials to access the database. A job running in our Nomad cluster
will only be allowed to use the `access-tables` policy.

If you would like to allow all policies to be used by any job in the Nomad
cluster except for the ones you specifically prohibit, then use the
`disallowed_policies` key instead and simply list the policies that should not
be granted. If you take this approach, be sure to include `nomad-server` in the
disallowed policies group. An example of this is shown below:

```json
{
  "disallowed_policies": "nomad-server",
  "explicit_max_ttl": 0,
  "name": "nomad-cluster",
  "orphan": true,
  "period": 259200,
  "renewable": true
}
```
Save the policy in a file named `nomad-cluster-role.json` and create the token
role named `nomad-cluster`.

```shell
$ vault write /auth/token/roles/nomad-cluster @nomad-cluster-role.json
```
You should see the following output:

```shell
Success! Data written to: auth/token/roles/nomad-cluster
```

### Step 6: Generate the Token for the Nomad Server

Run the following command to create a token for your Nomad server:

```shell
$ vault token create -policy nomad-server -period 72h -orphan
```
The `-orphan` flag is included when generating the Nomad server token above to prevent revocation of the token when its parent expires. Vault typically creates tokens with a parent-child relationship. When an ancestor token is revoked, all of its descendant tokens and their associated leases are revoked as well.

If everything works, you should see output similar to the following:

```shell
Key                  Value
---                  -----
token                1gr0YoLyTBVZl5UqqvCfK9RJ
token_accessor       5fz20DuDbxKgweJZt3cMynya
token_duration       72h
token_renewable      true
token_policies       ["default" "nomad-server"]
identity_policies    []
policies             ["default" "nomad-server"]
```
### Step 7: Edit the Nomad Server Configuration to Enable Vault Integration

At this point, you are ready to edit the [vault stanza][vault-stanza] in the Nomad Server's configuration file located at `/etc/nomad.d/nomad.hcl`. Provide the token you generated in the previous step in the `vault` stanza of
your Nomad server configuration. The token can also be provided as an
environment variable called `VAULT_TOKEN`. Be sure to specify the
`nomad-cluster-role` in the [create_from_role][create-from-role] option. After
following these steps and enabling Vault, the `vault` stanza in your Nomad server configuration will be similar to what is shown below:

```hcl
vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
  task_token_ttl = "1h"
  create_from_role = "nomad-cluster"
  token = "<your nomad server token>"
}
```

Restart the Nomad server

```shell
$ sudo systemctl restart nomad
```

NOTE: Nomad servers will renew the token automatically. 

Vault integration needs to be enabled on the client nodes as well, but this has
been configured for you already in this environment. You will see the `vault`
stanza in your Nomad clients' configuration (located at `/etc/nomad.d/nomad.hcl`) looks similar to the following:

```hcl
vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
}
```
Please note that the Nomad clients do not need to be provided with a Vault
token.

### Step 8: Deploy Database

The next few steps will involve configuring a connection between Vault and our
database, so let's deploy one that we can connect to. Create a Nomad job called
`db.nomad` with the following content:

```hcl
job "postgres-nomad-demo" {
  datacenters = ["dc1"]

  group "db" {

    task "server" {
      driver = "docker"

      config {
        image = "hashicorp/postgres-nomad-demo:latest"
        port_map {
          db = 5432
        }
      }
      resources {
        network {
          port  "db"{
	    static = 5432
	  }
        }
      }

      service {
        name = "database"
        port = "db"

        check {
          type     = "tcp"
          interval = "2s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

Run the job as shown below:

```shell
$ nomad run db.nomad 
```

Verify the job is running with the following command:

```shell
$ nomad status postgres-nomad-demo
```

The result of the status command will look similar to the output below:

```shell
ID            = postgres-nomad-demo
Name          = postgres-nomad-demo
Submit Date   = 2018-11-15T21:01:00Z
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
db          0       0         1        0       0         0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created    Modified
701e2699  5de1330c  db          0        run      running  1m56s ago  1m33s ago
```

Now we can move on to configuring the connection between Vault and our database.

### Step 9: Enable the Database Secrets Engine

We are using the database secrets engine for Vault in this exercise so that we
can generate dynamic credentials for our PostgreSQL database. Run the following command to enable it:

```shell
$ vault secrets enable database
```
If the previous command was successful, you will see the following output:

```shell
Success! Enabled the database secrets engine at: database/
```

### Step 10: Configure the Database Secrets Engine

Create a file named `connection.json` and placed the following information into
it:

```json
{
  "plugin_name": "postgresql-database-plugin",
  "allowed_roles": "accessdb",
  "connection_url": "postgresql://{{username}}:{{password}}@database.service.consul:5432/postgres?sslmode=disable",
  "username": "postgres",
  "password": "postgres123"
}
```
The information above allows Vault to connect to our database and create users
with specific privileges. We will specify the `accessdb` role soon. In a
production setting, it is recommended to give Vault credentials with enough
privileges to generate database credentials dynamically and and manage their
lifecycle.

Run the following command to configure the connection between the database
secrets engine and our database:

```shell
$ vault write database/config/postgresql @connection.json
```

If the operation is successful, there will be no output.

### Step 11: Create a Vault Role to Manage Database Privileges

Recall from the previous step that we specified `accessdb` in the
`allowed_roles` key of our connection information. Let's set up that role now. Create a file called `accessdb.sql` with the following content:

```shell
CREATE USER "{{name}}" WITH ENCRYPTED PASSWORD '{{password}}' VALID UNTIL
'{{expiration}}';
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO "{{name}}";
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO "{{name}}"; 
GRANT ALL ON SCHEMA public TO "{{name}}";
```

The SQL above will be used in the [creation_statements][creation-statements]
parameter of our next command to specify the privileges that the dynamic
credentials being generated will possess. In our case, the dynamic database user
will have broad privileges that include the ability to read from the tables that
our application will need to access.

Run the following command to create the role:

```shell
$ vault write database/roles/accessdb db_name=postgresql \
creation_statements=@accessdb.sql default_ttl=1h max_ttl=24h
```
You should see the following output after running the previous command:

```shell
Success! Data written to: database/roles/accessdb
```

### Step 12: Generate PostgreSQL Credentials

You should now be able to generate dynamic credentials to access your database.
Run the following command to generate a set of credentials:

```shell
$ vault read database/creds/accessdb
```
The previous command should return output similar to what is shown below:

```shell
Key                Value
---                -----
lease_id           database/creds/accessdb/3JozEMSMqw0vHHhvla15sKTW
lease_duration     1h
lease_renewable    true
password           A1a-3pMGjpDXHZ2Qzuf7
username           v-root-accessdb-5LA65urB4daA8KYy2xku-1542318363
```
Congratulations! You have configured Vault's connection to your database and
can now generate credentials with the previously specified privileges. Now we need to deploy our application and make sure that it will be able to communicate with Vault and obtain the credentials as well.

### Step 13: Create the `access-tables` Policy for Your Nomad Job to Use

Recall from [Step 5][step-5] that we specified a policy named `access-tables` in
our `allowed_policies` section of the token role. We will create this policy now
and give it the capability to read from the `database/creds/accessdb` endpoint
(the same endpoint we read from in the previous step to generate credentials for
our database). We will then specify this policy in our Nomad job which will
allow it to retrieve credentials for itself to access the database.

On the Nomad server (which is also running Vault), create a file named `access-tables-policy.hcl` with the following content:

```hcl
path "database/creds/accessdb" {
  capabilities = ["read"]  
}
```
Create the `access-tables` policy with the following command:

```shell
$ vault policy write access-tables access-tables-policy.hcl 
```
You should see the following output:

```shell
Success! Uploaded policy: access-tables
```

### Step 14: Deploy Your Job with the Appropriate Policy and Templating

Now we are ready to deploy our web application and give it the necessary policy
and configuration to communicate with our database. Create a file called
`web-app.nomad` and save the following content in it.

```hcl
job "nomad-vault-demo" {
  datacenters = ["dc1"]

  group "demo" {
    task "server" {

      vault {
        policies = ["access-tables"]
      }

      driver = "docker"
      config {
        image = "hashicorp/nomad-vault-demo:latest"
        port_map {
          http = 8080
        }

        volumes = [
          "secrets/config.json:/etc/demo/config.json"
        ]
      }

      template {
        data = <<EOF
{{ with secret "database/creds/accessdb" }}
  {
    "host": "database.service.consul",
    "port": 5432,
    "username": "{{ .Data.username }}",
    "password": "{{ .Data.password }}",
    "db": "postgres"
  }
{{ end }}
EOF
        destination = "secrets/config.json"
      }

      resources {
        network {
          port "http" {}
        }
      }

      service {
        name = "nomad-vault-demo"
        port = "http"

        tags = [
          "urlprefix-/",
        ]

        check {
          type     = "tcp"
          interval = "2s"
          timeout  = "2s"
        }
      }
    }
  }
}
```
There are a few key points to note here:

- We have specified the `access-tables` policy in the [vault][vault-jobspec]
  stanza of this job. The Nomad client will receive a token with this policy
attached. Recall from the previous step that this policy will allow our
application to read from the `database/creds/accessdb` endpoint in Vault and retrieve credentials.
- We are using the [template][template] stanza's [vault
  integration][nomad-template-vault] to populate the JSON configuration file
that our application needs. The underlying tool being used is [Consul
Template][consul-template]. You can use Consul Template's documentation to learn
more about the [syntax][consul-temp-syntax] needed to interact with Vault.
Please note that although we have defined the template [inline][inline], we can use the template stanza [in conjunction with the artifact stanza][remote-template] to download an input template from a remote source such as an S3 bucket.
- Finally, note that that [destination][destination] of our template is the
  [secrets/][secrets-task-directory] task directory. This ensures the data is
not accessible with a command like [nomad alloc fs][nomad-alloc-fs] or
filesystem APIs.

Use the following command to run the job:

```shell
$ nomad run web-app.nomad 
```

### Step 15: Confirm the Application is Accessing the Database

At this point, you can visit your application at the path `/names` to confirm
the appropriate data is being accessed from the database and displayed to you.
There are several ways to do this.

- Use the `dig` command to query the SRV record of your service and obtain the
  port it is using. Then `curl` your service at the appropriate port and `names` path.

```shell
$ dig +short SRV nomad-vault-demo.service.consul
1 1 30478 ip-172-31-58-230.node.dc1.consul.
```
```shell
$ curl nomad-vault-demo.service.consul:30478/names
<!DOCTYPE html>
<html>
<body>

<h1> Welcome! </h1>
<h2> If everything worked correctly, you should be able to see a list of names below </h2>

<hr>


<h4> John Doe </h4>

<h4> Peter Parker </h4>

<h4> Clifford Roosevelt </h4>

<h4> Bruce Wayne </h4>

<h4> Steven Clark </h4>

<h4> Mary Jane </h4>


</body>
<html>
```
- You can also deploy [fabio][fabio] and visit any Nomad client at its public IP
  address using a fixed port. The details of this method are beyond the scope of
this guide, but you can refer to the [Load Balancing with Fabio][fabio-lb] guide
for more information on this topic. Alternatively, you could use the `nomad`
[alloc status][alloc-status] command along with the AWS console to determine the
public IP and port your service is running (remember to open the port in your
AWS security group if you choose this method).

[![Web Service][web-service]][web-service]

[alloc-status]: /docs/commands/alloc/status.html
[consul-template]: https://github.com/hashicorp/consul-template
[consul-temp-syntax]: https://github.com/hashicorp/consul-template#secret
[create-from-role]: /docs/configuration/vault.html#create_from_role
[creation-statements]: https://www.vaultproject.io/api/secret/databases/index.html#creation_statements
[destination]: /docs/job-specification/template.html#destination
[fabio]: https://github.com/fabiolb/fabio
[fabio-job]: /guides/load-balancing/fabio.html#step-1-create-a-job-for-fabio
[fabio-lb]: /guides/load-balancing/fabio.html
[inline]: /docs/job-specification/template.html#inline-template
[login]: https://www.vaultproject.io/docs/commands/login.html
[nomad-alloc-fs]: /docs/commands/alloc/fs.html
[nomad-template-vault]: /docs/job-specification/template.html#vault-integration
[policy]: https://www.vaultproject.io/docs/concepts/policies.html
[postgresql]: https://www.postgresql.org/about/
[remote-template]: /docs/job-specification/template.html#remote-template
[repo]: https://github.com/hashicorp/nomad/tree/master/terraform
[role]: https://www.vaultproject.io/docs/auth/token.html
[seal]: https://www.vaultproject.io/docs/concepts/seal.html
[secrets-task-directory]: /docs/runtime/environment.html#secrets-
[step-5]: /guides/operations/vault-integration/index.html#step-5-create-a-token-role
[template]: /docs/job-specification/template.html
[token]: https://www.vaultproject.io/docs/concepts/tokens.html
[vault]: https://www.vaultproject.io/
[vault-integration]: /docs/vault-integration/index.html
[vault-jobspec]: /docs/job-specification/vault.html
[vault-stanza]: /docs/configuration/vault.html
[web-service]: /assets/images/nomad-demo-app.png
