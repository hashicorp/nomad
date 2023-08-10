/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import {
  fragmentOwner,
  fragmentArray,
  fragment,
} from 'ember-data-model-fragments/attributes';
import sumAggregation from '../utils/properties/sum-aggregation';
import classic from 'ember-classic-decorator';

const maybe = (arr) => arr || [];

@classic
export default class TaskGroup extends Fragment {
  @fragmentOwner() job;

  @attr('string') name;
  @attr('number') count;

  @computed('job.{variables,parent,plainId}', 'name')
  get pathLinkedVariable() {
    if (this.job.parent.get('id')) {
      return this.job.variables?.findBy(
        'path',
        `nomad/jobs/${this.job.parent.get('plainId')}/${this.name}`
      );
    } else {
      return this.job.variables?.findBy(
        'path',
        `nomad/jobs/${this.job.plainId}/${this.name}`
      );
    }
  }

  // TODO: This async fetcher seems like a better fit for most of our use-cases than the above getter (which cannot do async/await)
  async getPathLinkedVariable() {
    await this.job.variables;
    if (this.job.parent.get('id')) {
      return await this.job.variables?.findBy(
        'path',
        `nomad/jobs/${this.job.parent.get('plainId')}/${this.name}`
      );
    } else {
      return await this.job.variables?.findBy(
        'path',
        `nomad/jobs/${this.job.plainId}/${this.name}`
      );
    }
  }

  @fragmentArray('task') tasks;

  @fragmentArray('service-fragment') services;

  @fragmentArray('volume-definition') volumes;

  @fragment('group-scaling') scaling;

  @attr() meta;

  @computed('job.meta.raw', 'meta')
  get mergedMeta() {
    return {
      ...this.job.get('meta.raw'),
      ...this.meta,
    };
  }

  @computed('tasks.@each.driver')
  get drivers() {
    return this.tasks.mapBy('driver').uniq();
  }

  @computed('job.allocations.{@each.taskGroup,isFulfilled}', 'name')
  get allocations() {
    return maybe(this.get('job.allocations')).filterBy(
      'taskGroupName',
      this.name
    );
  }

  @sumAggregation('tasks', 'reservedCPU') reservedCPU;
  @sumAggregation('tasks', 'reservedMemory') reservedMemory;
  @sumAggregation('tasks', 'reservedDisk') reservedDisk;

  @computed('tasks.@each.{reservedMemory,reservedMemoryMax}')
  get reservedMemoryMax() {
    return this.get('tasks')
      .map((t) => t.get('reservedMemoryMax') || t.get('reservedMemory'))
      .reduce((sum, count) => sum + count, 0);
  }

  @attr('number') reservedEphemeralDisk;

  @computed('job.latestFailureEvaluation.failedTGAllocs.[]', 'name')
  get placementFailures() {
    const placementFailures = this.get(
      'job.latestFailureEvaluation.failedTGAllocs'
    );
    return placementFailures && placementFailures.findBy('name', this.name);
  }

  @computed('summary.{queuedAllocs,startingAllocs}')
  get queuedOrStartingAllocs() {
    return (
      this.get('summary.queuedAllocs') + this.get('summary.startingAllocs')
    );
  }

  @computed('job.taskGroupSummaries.[]', 'name')
  get summary() {
    return maybe(this.get('job.taskGroupSummaries')).findBy('name', this.name);
  }

  @computed('job.scaleState.taskGroupScales.[]', 'name')
  get scaleState() {
    return maybe(this.get('job.scaleState.taskGroupScales')).findBy(
      'name',
      this.name
    );
  }

  scale(count, message) {
    return this.job.scale(this.name, count, message);
  }
}
