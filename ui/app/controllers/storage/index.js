/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import Controller from '@ember/controller';
import { scheduleOnce } from '@ember/runloop';
import { restartableTask, timeout } from 'ember-concurrency';
import Ember from 'ember';

const TASK_THROTTLE = 1000;

export default class IndexController extends Controller {
  @service router;
  @service userSettings;
  @service system;
  @service keyboard;

  queryParams = [
    { qpNamespace: 'namespace' },
    'dhvPage',
    'csiPage',
    'dhvFilter',
    'csiFilter',
    'dhvSortProperty',
    'csiSortProperty',
    'dhvSortDescending',
    'csiSortDescending',
  ];

  @tracked qpNamespace = '*';

  pageSizes = [10, 25, 50];

  get optionsNamespaces() {
    const availableNamespaces = this.model.namespaces.map((namespace) => ({
      key: namespace.name,
      label: namespace.name,
    }));

    availableNamespaces.unshift({
      key: '*',
      label: 'All (*)',
    });

    // Unset the namespace selection if it was server-side deleted
    if (!availableNamespaces.mapBy('key').includes(this.qpNamespace)) {
      // eslint-disable-next-line ember/no-incorrect-calls-with-inline-anonymous-functions
      scheduleOnce('actions', () => {
        // eslint-disable-next-line ember/no-side-effects
        this.qpNamespace = '*';
      });
    }

    return availableNamespaces;
  }

  get dhvColumns() {
    return [
      {
        key: 'plainId',
        label: 'ID',
        isSortable: true,
      },
      {
        key: 'name',
        label: 'Name',
        isSortable: true,
      },
      ...(this.system.shouldShowNamespaces
        ? [
            {
              key: 'namespace',
              label: 'Namespace',
              isSortable: true,
            },
          ]
        : []),
      {
        key: 'node.name',
        label: 'Node',
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
      {
        key: 'modifyTime',
        label: 'Last Modified',
        isSortable: true,
      },
    ];
  }

  get csiColumns() {
    let cols = [
      {
        key: 'plainId',
        label: 'ID',
        isSortable: true,
      },
      ...(this.system.shouldShowNamespaces
        ? [
            {
              key: 'namespace',
              label: 'Namespace',
              isSortable: true,
            },
          ]
        : []),
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
        key: 'plugin.plainId',
        label: 'Plugin',
      },
      {
        key: 'allocationCount',
        label: '# Allocs',
        isSortable: true,
      },
    ].filter(Boolean);
    return cols;
  }

  // For all volume types:
  // Filter, then Sort, then Paginate
  // all handled client-side

  @tracked csiVolumes = this.model.csiVolumes;
  get filteredCSIVolumes() {
    if (!this.csiFilter) {
      return this.csiVolumes;
    } else {
      return this.csiVolumes.filter((volume) => {
        return (
          volume.plainId.toLowerCase().includes(this.csiFilter.toLowerCase()) ||
          volume.name.toLowerCase().includes(this.csiFilter.toLowerCase())
        );
      });
    }
  }

  get sortedCSIVolumes() {
    let sorted = this.filteredCSIVolumes.sortBy(this.csiSortProperty);
    if (this.csiSortDescending) {
      sorted.reverse();
    }
    return sorted;
  }

  get paginatedCSIVolumes() {
    return this.sortedCSIVolumes.slice(
      (this.csiPage - 1) * this.userSettings.pageSize,
      this.csiPage * this.userSettings.pageSize
    );
  }

  @tracked dynamicHostVolumes = this.model.dynamicHostVolumes;
  get filteredDynamicHostVolumes() {
    if (!this.dhvFilter) {
      return this.dynamicHostVolumes;
    } else {
      return this.dynamicHostVolumes.filter((volume) => {
        return (
          volume.plainId.toLowerCase().includes(this.dhvFilter.toLowerCase()) ||
          volume.name.toLowerCase().includes(this.dhvFilter.toLowerCase())
        );
      });
    }
  }

  get sortedDynamicHostVolumes() {
    let sorted = this.filteredDynamicHostVolumes.sortBy(this.dhvSortProperty);
    if (this.dhvSortDescending) {
      sorted.reverse();
    }
    return sorted;
  }

  get paginatedDynamicHostVolumes() {
    return this.sortedDynamicHostVolumes.slice(
      (this.dhvPage - 1) * this.userSettings.pageSize,
      this.dhvPage * this.userSettings.pageSize
    );
  }

  @tracked csiSortProperty = 'id';
  @tracked csiSortDescending = false;
  @tracked csiPage = 1;
  @tracked csiFilter = '';

  @tracked dhvSortProperty = 'modifyTime';
  @tracked dhvSortDescending = true;
  @tracked dhvPage = 1;
  @tracked dhvFilter = '';

  @action handlePageChange(type, page) {
    if (type === 'csi') {
      this.csiPage = page;
    } else if (type === 'dhv') {
      this.dhvPage = page;
    }
  }

  @action handleSort(type, sortBy, sortOrder) {
    this[`${type}SortProperty`] = sortBy;
    this[`${type}SortDescending`] = sortOrder === 'desc';
  }

  @action applyFilter(type, event) {
    this[`${type}Filter`] = event.target.value;
    this[`${type}Page`] = 1;
  }

  @action openCSI(csi) {
    this.router.transitionTo('storage.volumes.volume', csi.idWithNamespace);
  }

  @action openDHV(dhv) {
    this.router.transitionTo(
      'storage.volumes.dynamic-host-volume',
      dhv.idWithNamespace
    );
  }

  @restartableTask *watchDHV(
    params,
    throttle = Ember.testing ? 0 : TASK_THROTTLE
  ) {
    while (true) {
      const abortController = new AbortController();
      try {
        const result = yield this.store.query('dynamic-host-volume', params, {
          reload: true,
          adapterOptions: {
            watch: true,
            abortController: abortController,
          },
        });

        this.dynamicHostVolumes = result;
      } catch (e) {
        console.error('Error fetching dynamic host volumes:', e);
        yield timeout(throttle);
      } finally {
        abortController.abort();
      }

      yield timeout(throttle);

      if (Ember.testing) {
        break;
      }
    }
  }

  @restartableTask *watchCSI(
    params,
    throttle = Ember.testing ? 0 : TASK_THROTTLE
  ) {
    while (true) {
      const abortController = new AbortController();
      try {
        const result = yield this.store.query('volume', params, {
          reload: true,
          adapterOptions: {
            watch: true,
            abortController: abortController,
          },
        });

        this.csiVolumes = result;
      } catch (e) {
        console.error('Error fetching CSI volumes:', e);
        yield timeout(throttle);
      } finally {
        abortController.abort();
      }

      yield timeout(throttle);

      if (Ember.testing) {
        break;
      }
    }
  }

  @action
  cancelQueryWatch() {
    this.watchDHV.cancelAll();
    this.watchCSI.cancelAll();
  }

  // (called from route)
  @action
  startQueryWatch(dhvQuery, csiQuery) {
    this.watchDHV.perform(dhvQuery.queryParams);
    this.watchCSI.perform(csiQuery.queryParams);
  }
}
