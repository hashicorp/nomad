import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';

@classic
export default class IndexController extends Controller.extend(WithNamespaceResetting) {
  @service system;
  @service store;

  @jobClientStatus('nodes', 'job') jobClientStatus;

  // TODO: use watch
  get nodes() {
    return this.store.peekAll('node');
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
