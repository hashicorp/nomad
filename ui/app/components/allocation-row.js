import Ember from 'ember';
import { lazyClick } from '../helpers/lazy-click';

const { Component, inject, run } = Ember;

export default Component.extend({
  store: inject.service(),

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
    // TODO: Use this code again once the temporary workaround below
    // is resolved.

    // If the job for this allocation is incomplete, reload it to get
    // detailed information.
    // const allocation = this.get('allocation');
    // if (
    //   allocation &&
    //   allocation.get('job') &&
    //   !allocation.get('job.isPending') &&
    //   !allocation.get('taskGroup')
    // ) {
    //   const job = allocation.get('job.content');
    //   job && job.reload();
    // }

    // TEMPORARY: https://github.com/emberjs/data/issues/5209
    // Ember Data doesn't like it when relationships aren't reflective,
    // which means the allocation's job will be null if it hasn't been
    // resolved through the allocation (allocation.get('job')) before
    // being resolved through the store (store.findAll('job')). The
    // workaround is to persist the jobID as a string on the allocation
    // and manually re-link the two records here.
    run.scheduleOnce('afterRender', this, qualifyJob);
  },
});

function qualifyJob() {
  const allocation = this.get('allocation');
  if (allocation.get('originalJobId')) {
    const job = this.get('store').peekRecord('job', allocation.get('originalJobId'));
    if (job) {
      allocation.setProperties({
        job,
        originalJobId: null,
      });
    } else {
      this.get('store')
        .findRecord('job', allocation.get('originalJobId'))
        .then(job => {
          allocation.set('job', job);
        });
    }
  }
}
