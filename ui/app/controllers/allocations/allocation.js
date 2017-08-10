import Ember from 'ember';
import Sortable from 'nomad-ui/mixins/sortable';

const { Controller, computed } = Ember;

export default Controller.extend(Sortable, {
  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'name',
  sortDescending: false,

  listToSort: computed.alias('model.states'),
  sortedStates: computed.alias('listSorted'),
});
