# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/server1"

# Enable the server
server {
    enabled = true

    # Self-elect, should be 3 or 5 for production
    bootstrap_expect = 1
}

vault {
	address = "https://10.0.0.231:8200"
	token = "6e073f4b-4a6d-1fde-812e-7ff65dd3f4fa"
	#allow_unauthenticated = true
	task_token_ttl = "5m"
	#enabled = true
	#tls_ca_file = "/etc/ssl/cluster/ca.pem"
	#tls_ca_path = "/etc/ssl/cluster"
	#tls_cert_file = "/etc/ssl/cluster/cert.pem"
	#tls_key_file = "/etc/ssl/cluster/key.pem"
	tls_server_name = "vault"
	tls_skip_verify = true
}
