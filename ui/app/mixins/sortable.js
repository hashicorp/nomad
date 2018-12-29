import Mixin from '@ember/object/mixin';
import { computed } from '@ember/object';

/**
  Sortable mixin

  Simple sorting behavior for a list of objects.

  Properties to override:
    - sortProperty: the property to sort by
    - sortDescending: when true, the list is reversed
    - listToSort: the list of objects to sort

  Properties provided:
    - listSorted: a copy of listToSort that has been sorted
*/
export default Mixin.create({
  // Override in mixin consumer
  sortProperty: null,
  sortDescending: true,
  listToSort: computed(() => []),

  listSorted: computed('listToSort.[]', 'sortProperty', 'sortDescending', function() {
    const sorted = this.get('listToSort')
      .compact()
      .sortBy(this.get('sortProperty'));
    if (this.get('sortDescending')) {
      return sorted.reverse();
    }
    return sorted;
  }),
});
