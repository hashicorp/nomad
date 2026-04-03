/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import { compare } from '@ember/utils';
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
    return [...this.model.allocations]
      .sort((a, b) => compare(get(a, 'modifyIndex'), get(b, 'modifyIndex')))
      .reverse();
  }

  @action gotoAllocation(allocation) {
    this.transitionToRoute('allocations.allocation', allocation.id);
  }
}
