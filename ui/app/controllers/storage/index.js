/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';
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

  @tracked sortedEphemeralDisks = [];
  @tracked sortedStaticHostVolumes = [];

  @action async scanForEphemeralDisks() {
    console.log('scanForEphemeralDisks');
    // Check store for all jobs
    const allAllocs = await this.store.findAll('allocation');
    console.log('allAllocs', allAllocs);
    const allocDataDirs = await Promise.all(
      allAllocs.map(async (alloc) => {
        // TODO: /logs is demo-only, /data is actual.
        // return alloc.ls('alloc/logs');
        return {
          // job: alloc.get('jobID'),
          // allocID: alloc.id,
          alloc,
          files: await alloc.ls('alloc/logs'),
        };
      })
    );
    console.log('allocDataDirs', allocDataDirs);
    this.sortedEphemeralDisks = allocDataDirs.map((dir, index) => {
      const files = dir.files;
      console.log('files', files);
      const totalFileSize = files.reduce((acc, file) => {
        return acc + file.Size;
      }, 0);
      console.log('indexed', index, allAllocs.objectAt(index));
      return {
        // job: allAllocs.objectAt(index).get('job.name'),
        // job: dir.job,
        name: dir.alloc.name,
        id: dir.alloc.id,
        size: totalFileSize,
      };
    });
    // console.log('sortedEphemeralDisks', sortedEphemeralDisks);
    // const allocsWithEphemeralDisks = allAllocs.filter((alloc) => {
    //   return alloc.job.ephemeralDisk;
    // });
    // console.log('allAllocs', allAllocs);
    // console.log('allocsWithEphemeralDisks', allocsWithEphemeralDisks);
  }

  @action scanForStaticHostVolumes() {
    console.log('scanForStaticHostVolumes');
  }
}
