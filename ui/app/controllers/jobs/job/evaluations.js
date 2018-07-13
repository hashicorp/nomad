import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import Sortable from 'nomad-ui/mixins/sortable';

export default Controller.extend(WithNamespaceResetting, Sortable, {
  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'modifyIndex',
  sortDescending: true,

  job: alias('model'),
  evaluations: alias('model.evaluations'),

  listToSort: alias('evaluations'),
  sortedEvaluations: alias('listSorted'),
});
