import Controller from '@ember/controller';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';
import { jobCrumbs } from 'nomad-ui/utils/breadcrumb-utils';

export default class AllocationsAllocationController extends Controller {
  // Allocation breadcrumbs extend from job / task group breadcrumbs
  // even though the route structure does not.
  get breadcrumbs() {
    const model = this.model;
    const jobQueryParams = qpBuilder({
      jobNamespace: model.get('job.namespace.name') || 'default',
    });

    return [
      { label: 'Jobs', args: ['jobs.index', jobQueryParams] },
      ...jobCrumbs(model.get('job')),
      {
        label: model.get('taskGroupName'),
        args: [
          'jobs.job.task-group',
          model.get('job.plainId'),
          model.get('taskGroupName'),
          jobQueryParams,
        ],
      },
      {
        label: model.get('shortId'),
        args: ['allocations.allocation', model],
      },
    ];
  }
}
