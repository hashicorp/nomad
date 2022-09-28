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

  get allocationPeers() {
    return this.job.taskGroups
      .filterBy('name', this.allocation.taskGroup.name)
      .reduce((allocs, group) => {
        return allocs.concat(
          group.allocations.rejectBy('id', this.allocation.id).map((alloc) => {
            return {
              label: alloc.shortId,
              args: ['allocations.allocation', alloc.id],
            };
          })
        );
      }, []);

    // console.log('getting allocpeers', this.allocation, this.store.peekAll('allocation'));
    // return this.store
    //   .peekAll('allocation')
    //   .rejectBy('id', this.allocation?.id)
    //   .map((alloc) => {
    //     return {
    //       label: alloc.shortId,
    //       args: ['allocations.allocation', alloc.id],
    //     };
    //   });
  }

  get taskGroupPeers() {
    const groupsWithinJob = this.job.taskGroups.map((group) => {
      return {
        label: group.name,
        args: ['jobs.job.task-group', this.job.idWithNamespace, group.name],
      };
    });
    return groupsWithinJob.length > 1 ? groupsWithinJob : null;
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
        peers: this.taskGroupPeers,
      },
      {
        title: 'Allocation',
        label: allocation.shortId,
        args: ['allocations.allocation', allocation],
        peers: this.allocationPeers,
      },
    ];
  }
}
