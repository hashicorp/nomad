/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  serializeIds: 'always',

  keyForRelationshipIds(relationship) {
    if (relationship === 'policies') {
      return 'Policies';
    }
    return ApplicationSerializer.prototype.keyForRelationshipIds.apply(this, arguments);
  },
});
