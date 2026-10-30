# Copyright IBM Corp. 2015, 2026
# SPDX-License-Identifier: BUSL-1.1

container {
  local_daemon = true

  secrets {
    all               = true
    skip_path_strings = ["/website/content/"]
  }

  dependencies    = true
  alpine_security = true
}

binary {
  go_modules = true
  osv        = true
  go_stdlib  = true
  nvd        = false

  secrets {
    all               = true
    skip_path_strings = ["/website/content/"]
  }

  # Triage items that are _safe_ to ignore here. Note that this list should be
  # periodically cleaned up to remove items that are no longer found by the scanner.
  triage {
    suppress {
      vulnerabilities = [
        "GO-2022-0635", // github.com/aws/aws-sdk-go@v1.55.6 TODO(jrasell): remove when dep updated.
        "GO-2025-3543", // github.com/opencontainers/runc    TODO(jrasell): remove once withdrawn from DBs.
        // Ref: https://pkg.go.dev/vuln/GO-2026-5932
        //
        // Exists in the "golang.org/x/crypto/openpgp" package which is not used
        // by Nomad and is deprecated and unmaintained.
        "GO-2026-5932",

        // github.com/go-jose/go-jose/v3 does not have a fixed version, so Nomad
        // needs to migrate from this library which needs to be roadmapped and
        // prioritized accordingly. These advisories are all for this dep
        "GHSA-mgcc-cww5-fp2q",
        "GHSA-4w3g-jfj2-cw2q",
        "GHSA-vvm8-gf78-28g4",
        "GHSA-6mp9-8xrw-74r4"
      ]
    }
  }
}
