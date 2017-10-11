import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';

const { Controller, computed, inject } = Ember;

export default Controller.extend(Sortable, {
  jobController: inject.controller('jobs.job'),

  queryParams: {
    currentPage: 'page',
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  currentPage: 1,
  pageSize: 10,

  sortProperty: 'name',
  sortDescending: false,

  breadcrumbs: computed.alias('jobController.breadcrumbs'),
  job: computed.alias('model'),

  taskGroups: computed('model.taskGroups.[]', function() {
    return this.get('model.taskGroups') || [];
  }),

  listToSort: computed.alias('taskGroups'),
  sortedTaskGroups: computed.alias('listSorted'),

  actions: {
    gotoTaskGroup(taskGroup) {
      this.transitionToRoute('jobs.job.task-group', taskGroup.get('job'), taskGroup);
    },
  },
});
