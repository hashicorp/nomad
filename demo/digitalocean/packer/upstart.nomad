description "Nomad by HashiCorp"

start on runlevel [2345]
stop on runlevel [!2345]

respawn

script
    exec /usr/local/bin/nomad -config /usr/local/etc/nomad.hcl >> /var/log/nomad.log 2>&1
end script
