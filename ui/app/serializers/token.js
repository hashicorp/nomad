/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { copy } from 'ember-copy';
import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class TokenSerializer extends ApplicationSerializer {
  primaryKey = 'AccessorID';

  attrs = {
    secret: 'SecretID',
  };

  normalize(typeHash, hash) {
    hash.PolicyIDs = hash.Policies;
    hash.PolicyNames = copy(hash.Policies);
    hash.Roles = hash.Roles || [];
    hash.RoleIDs = hash.Roles.map((role) => role.ID);
    return super.normalize(typeHash, hash);
  }

  serialize(snapshot, options) {
    const hash = super.serialize(snapshot, options);

    if (snapshot.id) {
      hash.AccessorID = snapshot.id;
    }

    delete hash.ExpirationTime;
    delete hash.CreateTime;
    delete hash.SecretID;

    hash.Policies = hash.PolicyIDs || [];
    delete hash.PolicyIDs;
    delete hash.PolicyNames;

    hash.Roles =
      (hash.RoleIDs || []).map((id) => {
        return { ID: id };
      }) || [];
    delete hash.RoleIDs;

    return hash;
  }
}
