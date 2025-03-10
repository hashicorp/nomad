/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default class DynamicHostVolumeController extends Controller {
  // Used in the template
  @service system;

  queryParams = [
    {
      volumeNamespace: 'namespace',
    },
  ];
  volumeNamespace = 'default';

  get volume() {
    return this.model;
  }

  get breadcrumbs() {
    const volume = this.volume;
    if (!volume) {
      return [];
    }
    return [
      {
        label: volume.name,
        args: [
          'storage.volumes.dynamic-host-volume',
          volume.plainId,
          qpBuilder({
            volumeNamespace: volume.get('namespace.name') || 'default',
          }),
        ],
      },
    ];
  }

  @computed('model.allocations.@each.modifyIndex')
  get sortedAllocations() {
    return this.model.allocations.sortBy('modifyIndex').reverse();
  }

  @action gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation.id);
  }
}
