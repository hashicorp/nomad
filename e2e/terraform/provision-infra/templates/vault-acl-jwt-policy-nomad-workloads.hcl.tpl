path "${MOUNT}/data/{{identity.entity.aliases.${AUTH_METHOD_ACCESSOR}.metadata.nomad_namespace}}/{{identity.entity.aliases.${AUTH_METHOD_ACCESSOR}.metadata.nomad_job_id}}/*" {
  capabilities = ["read"]
}

path "${MOUNT}/data/{{identity.entity.aliases.${AUTH_METHOD_ACCESSOR}.metadata.nomad_namespace}}/{{identity.entity.aliases.${AUTH_METHOD_ACCESSOR}.metadata.nomad_job_id}}" {
  capabilities = ["read"]
}

path "${MOUNT}/metadata/{{identity.entity.aliases.${AUTH_METHOD_ACCESSOR}.metadata.nomad_namespace}}/*" {
  capabilities = ["list"]
}

path "${MOUNT}/metadata/*" {
  capabilities = ["list"]
}
