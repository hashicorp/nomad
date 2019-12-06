import Mixin from '@ember/object/mixin';
import { computed } from '@ember/object';

/**
  Sortable mixin factory

  Simple sorting behavior for a list of objects. Pass the list of properties
  you want the list to be live-sorted based on, or use the generic sortable.js
  if you don’t need that.

  Properties to override:
    - sortProperty: the property to sort by
    - sortDescending: when true, the list is reversed
    - listToSort: the list of objects to sort

  Properties provided:
    - listSorted: a copy of listToSort that has been sorted
*/
export default function sortableFactory(properties) {
  const eachProperties = properties.map(property => `listToSort.@each.${property}`);

  return Mixin.create({
    // Override in mixin consumer
    sortProperty: null,
    sortDescending: true,
    listToSort: computed(() => []),

    listSorted: computed(
      ...eachProperties,
      'listToSort.[]',
      'sortProperty',
      'sortDescending',
      function() {
        const sorted = this.listToSort.compact().sortBy(this.sortProperty);
        if (this.sortDescending) {
          return sorted.reverse();
        }
        return sorted;
      }
    ),
  });
}
