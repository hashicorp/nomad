/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  extends: 'recommended',
  rules: {
    'require-presentational-children': 'off',
    'no-array-prototype-extensions': 'off',
    'no-builtin-form-components': 'off',
    'no-at-ember-render-modifiers': 'off',
    'link-href-attributes': 'off',
    'no-invalid-interactive': 'off',
    'no-inline-styles': 'off',
    'no-curly-component-invocation': {
      allow: ['format-volume-name', 'keyboard-commands'],
    },
    'no-implicit-this': { allow: ['keyboard-commands'] },
  },
  ignore: [
    'app/components/breadcrumbs/*', // using {{(modifier)}} syntax
    'app/components/list-pagination/list-pager', // using {{(modifier)}} syntax
  ],
};
