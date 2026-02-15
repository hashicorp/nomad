# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Configuration for security scanner.
# Run on PRs and pushes to `main` and `release/**` branches.
# See .github/workflows/security-scan.yml for CI config.

# To run manually, install scanner and then run `scan repository .`

# Scan results are triaged via the GitHub Security tab for this repo.
# See `security-scanner` docs for more information on how to add `triage` config
# for specific results or to exclude paths.

# This file controls scanning the repository only, not release artifacts. See
# .release/security-scan.hcl for the scanner config for release artifacts, which
# will block releases.

repository {
  go_modules             = true
  npm                    = true
  osv                    = true
  go_stdlib_version_file = ".go-version"

  secrets {
    all               = true
    skip_path_strings = ["/website/content/"]
  }

  github_actions {
    pinned_hashes = true
  }

  dependabot {
    required     = true
    check_config = true
  }

  dockerfile {
    pinned_hashes = true
    curl_bash     = true
  }

  plugin "codeql" {
    languages = ["go"]
  }

  plugin "semgrep" {
    use_git_ignore = true

    exclude = [
      "*_test.go",
      "website/*",
      "e2e/*",
      "internal/testing/*",
      "changelog/*",
      "*.md",
    ]

    config = [
      "p/gosec",                                                    # https://semgrep.dev/p/gosec
      "p/r2c-security-audit",                                       # https://semgrep.dev/p/r2c-security-audit
      "p/github-actions",                                           # https://semgrep.dev/p/github-actions
      "r/trailofbits.go.hanging-goroutine.hanging-goroutine",       # https://semgrep.dev/p/trailofbits
      "r/trailofbits.go.racy-append-to-slice.racy-append-to-slice", # https://semgrep.dev/p/trailofbits
      "r/trailofbits.go.racy-write-to-map.racy-write-to-map",       # https://semgrep.dev/p/trailofbits
      ".semgrep/",
    ]
  }

  # Triage items that are _safe_ to ignore here. Note that this list should be
  # periodically cleaned up to remove items that are no longer found by the scanner.
  triage {
    suppress {
      vulnerabilities = [
        "GO-2025-3543", // github.com/opencontainers/runc TODO(jrasell): remove once withdrawn from DBs.
      ]

      paths = [
        "ui/tests/*",
        "internal/testing/*",
        "testutil/*",
        "website/content/*",
      ]
    }
  }
}
