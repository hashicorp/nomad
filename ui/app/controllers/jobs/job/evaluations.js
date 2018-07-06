import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import Sortable from 'nomad-ui/mixins/sortable';

export default Controller.extend(WithNamespaceResetting, Sortable, {
  jobController: controller('jobs.job'),

  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'modifyIndex',
  sortDescending: true,

  job: alias('model'),
  evaluations: alias('model.evaluations'),

  breadcrumbs: alias('jobController.breadcrumbs'),

  listToSort: alias('evaluations'),
  sortedEvaluations: alias('listSorted'),
});
