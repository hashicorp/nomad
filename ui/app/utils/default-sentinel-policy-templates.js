/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import foo from './sentinel_policy_templates/foo';

export default [
  {
    name: 'something',
    description: 'something',
    template: foo,
  },
  {
    name: 'something-else',
    description: 'something else',
    template: foo,
  },
];
