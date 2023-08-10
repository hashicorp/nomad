/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { create, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/variables'),
  visitNew: visitable('/variables/new'),
  visitConflicting: visitable(
    '/variables/var/Auto-conflicting%20Variable@default/edit'
  ),
});
