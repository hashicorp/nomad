/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';
import { copy } from 'ember-copy';

@classic
export default class RoleSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.Policies = hash.Policies || []; // null guard
    hash.PolicyIDs = hash.Policies.map((policy) => policy.Name);
    hash.PolicyNames = copy(hash.PolicyIDs);
    return super.normalize(typeHash, hash);
  }
  serialize(snapshot, options) {
    const hash = super.serialize(snapshot, options);
    // required for update/PUT requests
    if (snapshot.id) {
      hash.ID = snapshot.id;
    }
    hash.Policies = hash.PolicyIDs.map((policy) => {
      return {
        Name: policy,
      };
    });
    delete hash.PolicyIDs;
    delete hash.PolicyNames;
    return hash;
  }
}
