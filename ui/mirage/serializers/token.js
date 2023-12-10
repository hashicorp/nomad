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
    if (relationship === 'roles') {
      return 'Roles';
    }
    return ApplicationSerializer.prototype.keyForRelationshipIds.apply(
      this,
      arguments
    );
  },

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeToken);
    } else {
      serializeToken(json);
    }
    return json;
  },
});

function serializeToken(token) {
  token.Roles = (token.Roles || []).map((role) => {
    return { ID: role, Name: role };
  });
  return token;
}
