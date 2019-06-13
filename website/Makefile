VERSION?="0.3.39"

build:
	@echo "==> Starting build in Docker..."
	@docker run \
		--interactive \
		--rm \
		--tty \
		--volume "$(shell pwd):/website" \
		-e "ENV=production" \
		hashicorp/middleman-hashicorp:${VERSION} \
		bundle exec middleman build --verbose --clean

website:
	@echo "==> Starting website in Docker..."
	@docker run \
		--interactive \
		--rm \
		--tty \
		--publish "4567:4567" \
		--publish "35729:35729" \
		--volume "$(shell pwd):/website" \
		hashicorp/middleman-hashicorp:${VERSION}

test:
	@echo "==> Running website tests..."
	./scripts/test.sh ${VERSION}

ui-screenshots:
	@echo "==> Collecting UI screenshots..."
	# Build the screenshots image if it doesn't exist yet
	@if [[ "$$(docker images -q nomad-ui-screenshots 2> /dev/null)" == "" ]]; then \
		docker build --tag="nomad-ui-screenshots" ./scripts/screenshots; \
	fi
	@docker run \
		--rm \
		--volume "$(shell pwd)/scripts/screenshots/screenshots:/screenshots" \
		nomad-ui-screenshots

ui-screenshots-local:
	@echo "==> Collecting UI screenshots (local)..."
	@cd scripts/screenshots/src && SCREENSHOTS_DIR="../screenshots" node index.js

.PHONY: build website
