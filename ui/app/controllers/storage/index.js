/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import Controller from '@ember/controller';

export default class IndexController extends Controller {
  @service router;

  queryParams = ['qpNamespace'];

  qpNamespace = 'default';

  get sortedDynamicHostVolumes() {
    return this.model.dynamicHostVolumes.sortBy('name');
  }

  get sortedCSIVolumes() {
    return this.model.csiVolumes.sortBy('name');
  }

  get dhvColumns() {
    return [
      {
        key: 'name',
        label: 'Name',
        isSortable: true,
      },
      {
        key: 'namespace',
        label: 'Namespace',
        isSortable: true,
      },
      {
        key: 'pluginID',
        label: 'Plugin ID',
        isSortable: true,
      },
      {
        key: 'state',
        label: 'State',
        isSortable: true,
      },
    ];
  }

  get csiColumns() {
    return [
      {
        key: 'name',
        label: 'Name',
        isSortable: true,
      },
      {
        key: 'namespace.name',
        label: 'Namespace',
        isSortable: true,
      },
      {
        key: 'schedulable',
        label: 'Volume Health',
        isSortable: true,
      },
      {
        key: 'controllersHealthyProportion',
        label: 'Controller Health',
      },
      {
        key: 'nodesHealthyProportion',
        label: 'Node Health',
      },
      {
        key: 'provider',
        label: 'Provider',
      },
      {
        key: 'allocationCount',
        label: '# Allocs',
      },
    ];
  }
  @action openDHV(dhv) {
    this.router.transitionTo('storage.dhv', dhv.name);
  }
}
