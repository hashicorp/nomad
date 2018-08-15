import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { task } from 'ember-concurrency';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';
import { next } from '@ember/runloop';

export default Controller.extend({
  stage: computed('planOutput', function() {
    return this.get('planOutput') ? 'plan' : 'editor';
  }),

  plan: task(function*() {
    this.cancel();

    try {
      yield this.get('model').parse();
    } catch (err) {
      this.set('parseError', err);
    }

    try {
      const planOutput = yield this.get('model').plan();
      this.set('planOutput', planOutput);
    } catch (err) {
      this.set('planError', err);
    }
  }).drop(),

  submit: task(function*() {
    try {
      yield this.get('model').run();
      const id = this.get('model.plainId');
      const namespace = this.get('model.namespace.name') || 'default';
      // navigate to the new job page
      this.transitionToRoute('jobs.job', id, {
        queryParams: { jobNamespace: namespace },
      });
    } catch (err) {
      this.set('runError', err);
    }
  }),

  cancel() {
    this.set('planOutput', null);
    this.set('planError', null);
    this.set('parseError', null);
  },
});
