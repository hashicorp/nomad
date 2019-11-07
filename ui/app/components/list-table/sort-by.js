import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  tagName: 'th',

  attributeBindings: ['title'],

  // The prop that the table is currently sorted by
  currentProp: '',

  // The prop this sorter controls
  prop: '',

  classNames: ['is-selectable'],
  classNameBindings: ['isActive:is-active', 'sortDescending:desc:asc'],

  isActive: computed('currentProp', 'prop', function() {
    return this.currentProp === this.prop;
  }),

  shouldSortDescending: computed('sortDescending', 'isActive', function() {
    return !this.isActive || !this.sortDescending;
  }),
});
