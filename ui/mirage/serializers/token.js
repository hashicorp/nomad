/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
