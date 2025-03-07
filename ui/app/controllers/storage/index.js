/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import Controller from '@ember/controller';
import { scheduleOnce } from '@ember/runloop';

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

  get filteredDynamicHostVolumes() {
    if (!this.dhvFilter) {
      return this.model.dynamicHostVolumes;
    } else {
      return this.model.dynamicHostVolumes.filter((volume) => {
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
}
