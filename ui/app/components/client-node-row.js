import Ember from 'ember';
import { lazyClick } from '../helpers/lazy-click';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',
  classNames: ['client-node-row', 'is-interactive'],

  node: null,

  onClick() {},

  click(event) {
    lazyClick([this.get('onClick'), event]);
  },

  didReceiveAttrs() {
    // Reload the node in order to get detail information
    const node = this.get('node');
    if (node) {
      node.reload();
    }
  },
});
