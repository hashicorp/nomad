#! /bin/sh
# generate models
# TBD

# Combine files to single spec
# TBD

# Sync docs with spec
# TBD

docker run -it \
  -v "${PWD}/website/content/api-docs:/local" \
  sean0x42/markdown-extract \
  -i -f "List Jobs" /local/jobs.mdx

# Generate test client from spec
# docker run --rm -v "${PWD}:/local" openapitools/openapi-generator-cli batch /local/config.yaml
# Direct jar alternative - requires changes paths in config.yaml
# java -jar ~/goland/scratch/openapi-generator-cli.jar batch config.yaml