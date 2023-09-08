/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { attr, belongsTo } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class StorageNode extends Fragment {
  @fragmentOwner() plugin;

  @belongsTo('node') node;
  @attr('string') allocID;

  @attr('string') provider;
  @attr('string') version;
  @attr('boolean') healthy;
  @attr('string') healthDescription;
  @attr('date') updateTime;
  @attr('boolean') requiresControllerPlugin;
  @attr('boolean') requiresTopologies;

  @attr() nodeInfo;

  // Fragments can't have relationships, so provider a manual getter instead.
  async getAllocation() {
    return this.store.findRecord('allocation', this.allocID);
  }
}
