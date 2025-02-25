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

  get sortedCsiVolumes() {
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

  @action openDHV(dhv) {
    this.router.transitionTo('storage.dhv', dhv.name);
  }
}
