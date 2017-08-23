import Ember from 'ember';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',

  classNames: ['allocation-row'],

  allocation: null,

  // Used to determine whether the row should mention the node or the job
  context: null,
});
