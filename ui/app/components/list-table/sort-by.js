import Ember from 'ember';

const { Component, computed } = Ember;

export default Component.extend({
  tagName: 'th',

  // The prop that the table is currently sorted by
  currentProp: '',

  // The prop this sorter controls
  prop: '',

  classNameBindings: ['isActive:is-active', 'sortDescending:desc:asc'],

  isActive: computed('currentProp', 'prop', function() {
    return this.get('currentProp') === this.get('prop');
  }),

  shouldSortDescending: computed('sortDescending', 'isActive', function() {
    return !this.get('isActive') || !this.get('sortDescending');
  }),
});
