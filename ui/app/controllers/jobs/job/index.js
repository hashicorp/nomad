import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

export default Controller.extend(WithNamespaceResetting, {
  system: service(),

  queryParams: {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,

  job: alias('model'),

  sortProperty: 'name',
  sortDescending: false,

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
