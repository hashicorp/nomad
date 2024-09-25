/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import ApplicationSerializer from './application';
import { inject as service } from '@ember/service';

export default class VersionTagSerializer extends ApplicationSerializer {
  @service store;

  serialize(snapshot, options) {
    const hash = super.serialize(snapshot, options);
    hash.Version = hash.VersionNumber;
    return hash;
  }
}
