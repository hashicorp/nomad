import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import { computed } from '@ember/object';
import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class IndexController extends Controller.extend(WithNamespaceResetting) {
  @service system;
  @service store;

  @computed('job')
  get uniqueNodes() {
    // add datacenter filter
    const allocs = this.job.allocations;
    const nodes = allocs.mapBy('node');
    const uniqueNodes = nodes.uniqBy('id').toArray();
    return uniqueNodes.map(nodeId => {
      return {
        [nodeId.get('id')]: allocs
          .toArray()
          .filter(alloc => nodeId.get('id') === alloc.get('node.id'))
          .map(alloc => alloc.getProperties('clientStatus'))
          .map(alloc => alloc.clientStatus),
      };
    });
  }

  @computed('node')
  get totalNodes() {
    return this.store.peekAll('node').toArray().length;
  }

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
  ];

  currentPage = 1;

  @alias('model') job;

  sortProperty = 'name';
  sortDescending = false;

  @action
  gotoTaskGroup(taskGroup) {
    this.transitionToRoute('jobs.job.task-group', taskGroup.get('job'), taskGroup);
  }

  @action
  gotoJob(job) {
    this.transitionToRoute('jobs.job', job, {
      queryParams: { jobNamespace: job.get('namespace.name') },
    });
  }
}
