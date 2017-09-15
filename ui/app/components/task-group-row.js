import Ember from 'ember';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',

  classNames: ['task-group-row', 'is-interactive'],

  taskGroup: null,

  onClick() {},

  click(event) {
    this.get('onClick')(event);
  },
});
