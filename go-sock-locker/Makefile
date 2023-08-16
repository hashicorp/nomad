# Weird AL(yank-o)LOC(K) # TODO: use brand for demo

clean: stop-weblock stop-sidelock delete-policies delete-vars gc

build:
	sudo docker build -t sock-locker:local .

docker-run: build
	sudo docker run --rm -it sock-locker:local sh

run-%: policy-%
	nomad run -detach $*.nomad.hcl

stop-%:
	nomad stop -purge $* || true
	nomad system gc

policy-sidelock:
	cat sidelock-policy.hcl
	nomad acl policy apply -namespace default -job sidelock sidelock sidelock-policy.hcl

policy-weblock:
	cat web-policy.hcl
	nomad acl policy apply -namespace default -job weblock weblock web-policy.hcl

delete-policies:
	nomad acl policy list -json | jq -r '.[].Name' | while read -r p; do nomad acl policy delete $$p; done

delete-vars:
	nomad var list -out=terse | grep . | while read -r v; do nomad var purge $$v; done

gc:
	nomad system gc

.PHONY: api
api:
	rsync -avP ../api/ ./api/

.PHONY: its-all-fake-ok
