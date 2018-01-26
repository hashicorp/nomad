SHELL = bash

.PHONY: all
all: \
	ca.pem ca-key.pem ca.csr \
       	client.pem client-key.pem client.csr \
	dev.pem dev-key.pem dev.csr \
	server.pem server-key.pem server.csr \
       	user.pem user-key.pem user.csr user.pfx

.PHONY: bootstrap
bootstrap: ## Install dependencies
	@echo "==> Updating cfssl..."
	go get -u github.com/cloudflare/cfssl/cmd/...

clean: ## Remove generated files
	@echo "==> Removing generated files..."
	rm -f \
		ca.pem ca-key.pem ca.csr \
		client.pem client-key.pem client.csr \
		dev.pem dev-key.pem dev.csr \
		server.pem server-key.pem server.csr \
		user.pem user-key.pem user.csr user.pfx

# Generate Nomad certificate authority
ca.pem ca-key.pem ca.csr:
	@echo "==> Removing generated files..."
	cfssl gencert -initca ca-csr.json | cfssljson -bare ca

# Generate Nomad server certificate
server.pem server-key.pem server.csr:
	@echo "==> Generating Nomad server certificate..."
	cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=cfssl.json \
		-hostname="server.global.nomad,localhost,127.0.0.1" csr.json  \
		| cfssljson -bare server

# Generate Nomad client node certificate
client.pem client-key.pem client.csr:
	@echo "==> Generating Nomad client node certificate..."
	cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=cfssl.json \
		-hostname="client.global.nomad,localhost,127.0.0.1" csr.json  \
		| cfssljson -bare client

# Generate Nomad combined server and client node certificate
dev.pem dev-key.pem dev.csr:
	@echo "==> Generating Nomad server and client node certificate..."
	cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=cfssl.json \
		-hostname="server.global.nomad,client.global.nomad,localhost,127.0.0.1" csr.json  \
		| cfssljson -bare dev

# Generate certificates for users (CLI and browsers)
user.pem user-key.pem user.csr user.pfx:
	@echo "==> Generating Nomad user certificates..."
	cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=cfssl-user.json \
		csr.json  | cfssljson -bare user
	openssl pkcs12 -export -inkey user-key.pem -in user.pem -out user.pfx -password pass:
