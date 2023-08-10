/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class PolicySerializer extends ApplicationSerializer {
  primaryKey = 'Name';

  normalize(typeHash, hash) {
    hash.ID = hash.Name;
    return super.normalize(typeHash, hash);
  }

  serialize(snapshot, options) {
    const hash = super.serialize(snapshot, options);
    hash.ID = hash.Name;
    return hash;
  }
}
