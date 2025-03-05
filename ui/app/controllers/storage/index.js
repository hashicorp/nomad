/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import Controller from '@ember/controller';
import { restartableTask } from 'ember-concurrency';
import { reduceBytes } from 'nomad-ui/utils/units';
import { scheduleOnce } from '@ember/runloop';

export default class IndexController extends Controller {
  @service router;
  @service userSettings;
  @service system;

  queryParams = [
    { qpNamespace: 'namespace' },
    'dhvPage',
    'csiPage',
    'shvPage',
    'edPage',
    'dhvFilter',
    'csiFilter',
    'shvFilter',
    'edFilter',
    'dhvSortProperty',
    'csiSortProperty',
    'shvSortProperty',
    'edSortProperty',
    'dhvSortDescending',
    'csiSortDescending',
    'shvSortDescending',
    'edSortDescending',
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
        isSortable: true,
      },
    ].filter(Boolean);
    return cols;
  }

  // For all volume types:
  // Filter, then Sort, then Paginate
  // all handled client-side

  get filteredCSIVolumes() {
    if (!this.csiFilter) {
      return this.model.csiVolumes;
    } else {
      return this.model.csiVolumes.filter((volume) => {
        return volume.name.toLowerCase().includes(this.csiFilter.toLowerCase());
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

  get filteredDynamicHostVolumes() {
    if (!this.dhvFilter) {
      return this.model.dynamicHostVolumes;
    } else {
      return this.model.dynamicHostVolumes.filter((volume) => {
        return volume.name.toLowerCase().includes(this.dhvFilter.toLowerCase());
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

  get filteredEphemeralDisks() {
    if (!this.edFilter) {
      return this.fetchedEphemeralDisks;
    } else {
      return this.fetchedEphemeralDisks.filter((disk) => {
        return disk.name.toLowerCase().includes(this.edFilter.toLowerCase());
      });
    }
  }

  get sortedEphemeralDisks() {
    let sorted = this.filteredEphemeralDisks.sortBy(this.edSortProperty);
    if (this.edSortDescending) {
      sorted.reverse();
    }
    return sorted;
  }

  get paginatedEphemeralDisks() {
    return this.sortedEphemeralDisks.slice(
      (this.edPage - 1) * this.userSettings.pageSize,
      this.edPage * this.userSettings.pageSize
    );
  }

  get filteredStaticHostVolumes() {
    if (!this.shvFilter) {
      return this.fetchedStaticHostVolumes;
    } else {
      return this.fetchedStaticHostVolumes.filter((volume) => {
        return volume.name.toLowerCase().includes(this.shvFilter.toLowerCase());
      });
    }
  }

  get sortedStaticHostVolumes() {
    let sorted = this.filteredStaticHostVolumes.sortBy(this.shvSortProperty);
    if (this.shvSortDescending) {
      sorted.reverse();
    }
    return sorted;
  }

  get paginatedStaticHostVolumes() {
    return this.sortedStaticHostVolumes.slice(
      (this.shvPage - 1) * this.userSettings.pageSize,
      this.shvPage * this.userSettings.pageSize
    );
  }

  @tracked csiSortProperty = 'name';
  @tracked csiSortDescending = false;
  @tracked csiPage = 1;

  @tracked dhvSortProperty = 'name';
  @tracked dhvSortDescending = false;
  @tracked dhvPage = 1;

  @tracked shvSortProperty = 'name';
  @tracked shvSortDescending = false;
  @tracked shvPage = 1;

  @tracked edSortProperty = 'sizeBytes';
  @tracked edSortDescending = true;
  @tracked edPage = 1;

  @tracked csiFilter = '';
  @tracked dhvFilter = '';
  @tracked shvFilter = '';
  @tracked edFilter = '';

  @action handlePageChange(type, page) {
    if (type === 'csi') {
      this.csiPage = page;
    } else if (type === 'dhv') {
      this.dhvPage = page;
    } else if (type === 'shv') {
      this.shvPage = page;
    } else if (type === 'ed') {
      this.edPage = page;
    }
  }

  @action handleSort(type, sortBy, sortOrder) {
    this[`${type}SortProperty`] = sortBy;
    this[`${type}SortDescending`] = sortOrder === 'desc';
  }

  @action applyFilter(type, event) {
    console.log('applyFilter', type, event);
    this[`${type}Filter`] = event.target.value;
    this[`${type}Page`] = 1;
  }

  @action openDHV(dhv) {
    this.router.transitionTo('storage.dhv', dhv.name);
  }

  @tracked fetchedEphemeralDisks = [];
  @tracked fetchedStaticHostVolumes = [];

  @restartableTask *scanForEphemeralDisks() {
    // Reset filters and pagination
    this.edPage = 1;
    this.edFilter = '';
    this.edSortProperty = 'sizeBytes';
    this.edSortDescending = true;

    const allAllocs = yield this.store.query(
      'allocation',
      { namespace: this.qpNamespace },
      { reload: true }
    );
    const allocDataDirs = yield Promise.all(
      allAllocs.map(async (alloc) => {
        return {
          alloc,
          files: await alloc.ls('alloc/data'),
        };
      })
    );

    yield (this.fetchedEphemeralDisks = allocDataDirs.map((dir) => {
      const files = dir.files;
      const totalFileSize = files.reduce((acc, file) => {
        return acc + file.Size;
      }, 0);
      const [size, unit] = reduceBytes(totalFileSize);
      return {
        name: dir.alloc.name,
        id: dir.alloc.id,
        sizeBytes: totalFileSize,
        size: `${+size.toFixed(2)} ${unit}`,
      };
    }));
  }

  @restartableTask *scanForStaticHostVolumes() {
    // Reset filters and pagination
    this.shvPage = 1;
    this.shvFilter = '';
    this.shvSortProperty = 'name';
    this.shvSortDescending = false;

    const allNodes = yield this.store.query(
      'node',
      { namespace: this.qpNamespace },
      { reload: true }
    );

    yield (this.fetchedStaticHostVolumes = allNodes
      .map((node) => {
        const hostVolumes = node.hostVolumes.filter((volume) => {
          return !volume.volumeID;
        });
        return hostVolumes.map((volume) => {
          return {
            name: volume.name,
            path: volume.path,
            readOnly: volume.readOnly,
            nodeID: node.id,
          };
        });
      })
      .flat());
  }

  // keyboard-shortcut-friendly actions

  @action performScanForEphemeralDisks() {
    this.scanForEphemeralDisks.perform();
  }

  @action performScanForStaticHostVolumes() {
    this.scanForStaticHostVolumes.perform();
  }

  @action pauseScanForEphemeralDisks() {
    this.scanForEphemeralDisks.cancelAll();
  }

  @action pauseScanForStaticHostVolumes() {
    this.scanForStaticHostVolumes.cancelAll();
  }
}
