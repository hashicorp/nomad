import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { action } from '@ember/object';
import classic from 'ember-classic-decorator';

@classic
export default class IndexController extends Controller.extend(WithNamespaceResetting) {
  @service system;

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
