/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

export default class ServiceSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.AllocationID = hash.AllocID;
    hash.JobID = JSON.stringify([hash.JobID, hash.Namespace]);
    return super.normalize(typeHash, hash);
  }
}
