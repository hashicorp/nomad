import Component from '@ember/component';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import Sortable from 'nomad-ui/mixins/sortable';

export default Component.extend(Sortable, {
  job: null,

  classNames: ['boxed-section'],

  // Provide a value that is bound to a query param
  sortProperty: null,
  sortDescending: null,
  currentPage: null,

  // Provide an action with access to the router
  gotoJob() {},

  pageSize: 10,

  taskGroups: computed('job.taskGroups.[]', function() {
    return this.get('job.taskGroups') || [];
  }),

  children: computed('job.children.[]', function() {
    return this.get('job.children') || [];
  }),

  listToSort: alias('children'),
  sortedChildren: alias('listSorted'),
});
