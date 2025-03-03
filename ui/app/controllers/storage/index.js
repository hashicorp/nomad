/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import Controller from '@ember/controller';
import { restartableTask, timeout } from 'ember-concurrency';
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
      {
        key: 'modifyTime',
        label: 'Last Modified',
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

  @tracked sortedEphemeralDisks = [];
  @tracked sortedStaticHostVolumes = [];

  @restartableTask *scanForEphemeralDisks() {
    const allAllocs = yield this.store.findAll('allocation', { reload: true });
    const allocDataDirs = yield Promise.all(
      allAllocs.map(async (alloc) => {
        return {
          alloc,
          files: await alloc.ls('alloc/logs'),
        };
      })
    );
    // TODO: demonstrative timeout
    yield timeout(2000);

    yield (this.sortedEphemeralDisks = allocDataDirs.map((dir) => {
      const files = dir.files;
      const totalFileSize = files.reduce((acc, file) => {
        return acc + file.Size;
      }, 0);
      return {
        name: dir.alloc.name,
        id: dir.alloc.id,
        size: totalFileSize,
      };
    }));
    // TODO: demonstrative timeout
    yield timeout(2000);
  }

  @restartableTask *scanForStaticHostVolumes() {
    const allNodes = yield this.store.findAll('node', { reload: true });

    // TODO: demonstrative timeout
    yield timeout(2000);

    this.sortedStaticHostVolumes = allNodes
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
      .flat();
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
