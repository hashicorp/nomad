import Ember from 'ember';

const { Component, inject } = Ember;

export default Component.extend({
  router: inject.service(),

  tagName: 'tr',
  classNames: ['client-node-row'],

  node: null,

  click() {
    this.get('router').transitionTo('nodes.node', this.get('node'));
  },

  didReceiveAttrs() {
    // Reload the node in order to get detail information
    const node = this.get('node');
    if (node) {
      node.reload();
    }
  },
});
