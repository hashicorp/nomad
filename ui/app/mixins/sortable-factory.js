/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Mixin from '@ember/object/mixin';
import Ember from 'ember';
import { computed } from '@ember/object';
import { warn } from '@ember/debug';

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
export default function sortableFactory(properties, fromSortableMixin) {
  const eachProperties = properties.map(
    (property) => `listToSort.@each.${property}`
  );

  // eslint-disable-next-line ember/no-new-mixins
  return Mixin.create({
    // Override in mixin consumer
    sortProperty: null,
    sortDescending: true,
    listToSort: computed(function () {
      return [];
    }),

    _sortableFactoryWarningPrinted: false,

    listSorted: computed(
      ...eachProperties,
      '_sortableFactoryWarningPrinted',
      'listToSort.[]',
      'sortDescending',
      'sortProperty',
      function () {
        if (!this._sortableFactoryWarningPrinted && !Ember.testing) {
          let message =
            'Using SortableFactory without property keys means the list will only sort when the members change, not when any of their properties change.';

          if (fromSortableMixin) {
            message +=
              ' The Sortable mixin is deprecated in favor of SortableFactory.';
          }

          warn(message, properties.length > 0, {
            id: 'nomad.no-sortable-properties',
          });
          // eslint-disable-next-line ember/no-side-effects
          this.set('_sortableFactoryWarningPrinted', true);
        }

        const sorted = this.listToSort.compact().sortBy(this.sortProperty);
        if (this.sortDescending) {
          return sorted.reverse();
        }
        return sorted;
      }
    ),
  });
}
