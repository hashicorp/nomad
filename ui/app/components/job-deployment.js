import Ember from 'ember';

const { Component } = Ember;

export default Component.extend({
  classNames: ['job-deployment', 'boxed-section'],

  deployment: null,
  isOpen: false,
});
