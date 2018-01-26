import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import Sortable from 'nomad-ui/mixins/sortable';

export default Controller.extend(Sortable, {
  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'name',
  sortDescending: false,

  listToSort: alias('model.states'),
  sortedStates: alias('listSorted'),
});
