import Ember from 'ember';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',
  classNames: ['client-node-row'],

  node: null,

  didReceiveAttrs() {
    // Reload the node in order to get detail information
    const node = this.get('node');
    if (node) {
      node.reload();
    }
  },
});
