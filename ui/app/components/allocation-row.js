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

  didReceiveAttrs() {
    // If the job for this allocation is incomplete, reload it to get
    // detailed information.
    const allocation = this.get('allocation');
    if (
      allocation &&
      allocation.get('job') &&
      !allocation.get('job.isPending') &&
      !allocation.get('taskGroup')
    ) {
      const job = allocation.get('job.content');
      job && job.reload();
    }
  },
});
