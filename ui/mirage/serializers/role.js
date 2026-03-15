/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  serializeIds: 'always',

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeRole);
    } else {
      serializeRole(json);
    }
    return json;
  },
});

function serializeRole(role) {
  const policyIds = role.PolicyIDs || role.Policies || [];

  role.Policies = policyIds.map((policy) => {
    if (typeof policy === 'object') {
      return policy;
    }
    return { ID: policy, Name: policy };
  });
  delete role.PolicyIDs;
  return role;
}
