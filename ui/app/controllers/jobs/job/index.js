import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';
import Sortable from 'nomad-ui/mixins/sortable';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

export default Controller.extend(Sortable, WithNamespaceResetting, {
  system: service(),
  jobController: controller('jobs.job'),

  breadcrumbs: alias('jobController.breadcrumbs'),

  queryParams: {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: 10,

  sortProperty: 'name',
  sortDescending: false,

  job: alias('model'),

  taskGroups: computed('model.taskGroups.[]', function() {
    return this.get('model.taskGroups') || [];
  }),

  listToSort: alias('taskGroups'),
  sortedTaskGroups: alias('listSorted'),

  sortedEvaluations: computed('model.evaluations.@each.modifyIndex', function() {
    return (this.get('model.evaluations') || []).sortBy('modifyIndex').reverse();
  }),

  actions: {
    gotoTaskGroup(taskGroup) {
      this.transitionToRoute('jobs.job.task-group', taskGroup.get('job'), taskGroup);
    },
  },
});
