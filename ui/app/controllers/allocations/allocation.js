import Ember from 'ember';

const { Controller, computed } = Ember;

export default Controller.extend({
  queryParams: {
    sortProperty: 'sort',
    sortDescending: 'desc',
  },

  sortProperty: 'name',
  sortDescending: false,

  sortedStates: computed('model.states.[]', 'sortProperty', 'sortDescending', function() {
    const sorted = (this.get('model.states') || []).sortBy(this.get('sortProperty'));
    if (this.get('sortDescending')) {
      return sorted.reverse();
    }
    return sorted;
  }),
});
