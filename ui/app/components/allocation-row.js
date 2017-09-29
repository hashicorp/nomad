import Ember from 'ember';
import { lazyClick } from '../helpers/lazy-click';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',

  classNames: ['allocation-row', 'is-interactive'],

  allocation: null,

  // Used to determine whether the row should mention the node or the job
  context: null,

  onClick() {},

  click(event) {
    lazyClick([this.get('onClick'), event]);
  },
});
