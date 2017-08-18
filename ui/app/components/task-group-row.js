import Ember from 'ember';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',

  classNames: ['task-group-row'],

  taskGroup: null,
});
