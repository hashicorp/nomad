/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

module.exports = {
  ...require('@hashicorp/platform-cli/config/stylelint.config'),
  rules: {
    'selector-pseudo-class-no-unknown': [
      true,
      {
        ignorePseudoClasses: ['first', 'last'],
      },
    ],
  },
}
