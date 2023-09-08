/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

module.exports = {
  extends: 'recommended',
  rules: {
    'link-href-attributes': 'off',
    'no-action': 'off',
    'no-invalid-interactive': 'off',
    'no-inline-styles': 'off',
    'no-curly-component-invocation': {
      allow: ['format-volume-name', 'keyboard-commands'],
    },
    'no-implicit-this': { allow: ['keyboard-commands'] },
  },
  ignore: [
    'app/components/breadcrumbs/*', // using {{(modifier)}} syntax
    'app/templates/components/list-pagination/list-pager', // using {{(modifier)}} syntax
  ],
};
