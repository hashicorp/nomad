import Ember from 'ember';

const { Component } = Ember;

export default Component.extend({
  classNames: ['job-version', 'boxed-section'],

  version: null,
  isOpen: false,

  actions: {
    toggleDiff() {
      this.toggleProperty('isOpen');
    },
  },
});
