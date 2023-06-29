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
}
