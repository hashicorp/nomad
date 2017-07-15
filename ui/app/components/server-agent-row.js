import Ember from 'ember';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',
  classNames: ['server-agent-row'],
  classNameBindings: ['isActive'],

  agent: null,
  isActive: false,
});
