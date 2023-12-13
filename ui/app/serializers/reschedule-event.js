/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

import classic from 'ember-classic-decorator';

@classic
export default class RescheduleEvent extends ApplicationSerializer {
  separateNanos = ['Time'];

  normalize(typeHash, hash) {
    hash.PreviousAllocationId = hash.PrevAllocID ? hash.PrevAllocID : null;
    hash.PreviousNodeId = hash.PrevNodeID ? hash.PrevNodeID : null;

    return super.normalize(typeHash, hash);
  }
}
