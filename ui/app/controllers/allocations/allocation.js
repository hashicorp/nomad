/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default class AllocationsAllocationController extends Controller {
  @service store;

  get allocation() {
    return this.model;
  }

  get job() {
    const allocation = this.model;
    const jobId = allocation.belongsTo('job').id();
    const job = this.store.peekRecord('job', jobId);
    return job;
  }

  get jobNamespace() {
    const jobNamespaceId = this.job.belongsTo('namespace').id();

    return jobNamespaceId || 'default';
  }
  // Allocation breadcrumbs extend from job / task group breadcrumbs
  // even though the route structure does not.
  get breadcrumbs() {
    const { allocation, job, jobNamespace } = this;
    const jobQueryParams = qpBuilder({
      jobNamespace,
    });

    return [
      { label: 'Jobs', args: ['jobs.index', jobQueryParams] },
      { type: 'job', job: job },
      {
        title: 'Task Group',
        label: allocation.taskGroupName,
        args: [
          'jobs.job.task-group',
          job.idWithNamespace,
          allocation.taskGroupName,
        ],
      },
      {
        title: 'Allocation',
        label: allocation.shortId,
        args: ['allocations.allocation', allocation],
      },
    ];
  }
}
