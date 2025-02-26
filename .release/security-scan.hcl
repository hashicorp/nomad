# Copyright (c) HashiCorp, Inc.
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
      ]
    }
  }
}
