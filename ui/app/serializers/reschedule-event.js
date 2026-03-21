/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';


export default class RescheduleEvent extends ApplicationSerializer {
  separateNanos = ['Time'];

  normalize(typeHash, hash) {
    hash.PreviousAllocationId = hash.PrevAllocID ? hash.PrevAllocID : null;
    hash.PreviousNodeId = hash.PrevNodeID ? hash.PrevNodeID : null;

    return super.normalize(typeHash, hash);
  }
}
