/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import {
  fragment,
  fragmentArray,
  fragmentOwner,
} from 'ember-data-model-fragments/attributes';
import { computed } from '@ember/object';

export default class Task extends Fragment {
  @fragmentOwner() taskGroup;

  @attr('string') name;
  @attr('string') driver;
  @attr('string') kind;

  @attr() meta;

  @computed('taskGroup.mergedMeta', 'meta')
  get mergedMeta() {
    return {
      ...this.taskGroup.mergedMeta,
      ...this.meta,
    };
  }

  @fragment('lifecycle') lifecycle;

  @computed('lifecycle', 'lifecycle.sidecar')
  get lifecycleName() {
    if (this.lifecycle) {
      const { hook, sidecar } = this.lifecycle;

      if (hook === 'prestart') {
        return sidecar ? 'prestart-sidecar' : 'prestart-ephemeral';
      } else if (hook === 'poststart') {
        return sidecar ? 'poststart-sidecar' : 'poststart-ephemeral';
      } else if (hook === 'poststop') {
        return 'poststop';
      }
    }

    return 'main';
  }

  @attr('number') reservedMemory;
  @attr('number') reservedMemoryMax;
  @attr('number') reservedCPU;
  @attr('number') reservedDisk;
  @attr('number') reservedEphemeralDisk;
  @fragmentArray('service-fragment') services;

  @fragmentArray('volume-mount', { defaultValue: () => [] }) volumeMounts;

  async _fetchParentJob() {
    let job = await this.store.findRecord('job', this.taskGroup.job.id, {
      reload: true,
    });
    this._job = job;
  }

  get pathLinkedVariable() {
    if (!this._job) {
      this._fetchParentJob();
      return null;
    } else {
      let jobID = this._job.plainId;
      if (this._job.parent.get('plainId')) {
        jobID = this._job.parent.get('plainId');
      }
      return this._job.variables?.findBy(
        'path',
        `nomad/jobs/${jobID}/${this.taskGroup.name}/${this.name}`
      );
    }
  }
}
