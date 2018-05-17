import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

export default Controller.extend(WithNamespaceResetting, {
  system: service(),

  jobController: controller('jobs.job'),

  queryParams: {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,

  sortProperty: 'name',
  sortDescending: false,

  breadcrumbs: alias('jobController.breadcrumbs'),
  job: alias('model'),

  actions: {
    gotoTaskGroup(taskGroup) {
      this.transitionToRoute('jobs.job.task-group', taskGroup.get('job'), taskGroup);
    },

    gotoJob(job) {
      this.transitionToRoute('jobs.job', job, {
        queryParams: { jobNamespace: job.get('namespace.name') },
      });
    },
  },
});
