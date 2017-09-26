import Ember from 'ember';
import { lazyClick } from '../helpers/lazy-click';

const { Component } = Ember;

export default Component.extend({
  tagName: 'tr',

  classNames: ['task-group-row', 'is-interactive'],

  taskGroup: null,

  onClick() {},

  click(event) {
    lazyClick([this.get('onClick'), event]);
  },
});
