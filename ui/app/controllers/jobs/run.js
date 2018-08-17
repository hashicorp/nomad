import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default Controller.extend({
  store: service(),

  parseError: null,
  planError: null,
  runError: null,

  planOutput: null,

  showPlanMessage: localStorageProperty('nomadMessageJobPlan', true),
  showEditorMessage: localStorageProperty('nomadMessageJobEditor', true),

  stage: computed('planOutput', function() {
    return this.get('planOutput') ? 'plan' : 'editor';
  }),

  plan: task(function*() {
    this.reset();

    try {
      yield this.get('model').parse();
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not parse input';
      this.set('parseError', error);
      return;
    }

    try {
      yield this.get('model').plan();
      const plan = this.get('store').peekRecord('job-plan', this.get('model.id'));
      this.set('planOutput', plan);
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not plan job';
      this.set('planError', error);
    }
  }).drop(),

  submit: task(function*() {
    try {
      yield this.get('model').run();

      const id = this.get('model.plainId');
      const namespace = this.get('model.namespace.name') || 'default';

      this.reset();

      // navigate to the new job page
      this.transitionToRoute('jobs.job', id, {
        queryParams: { jobNamespace: namespace },
      });
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not submit job';
      this.set('runError', error);
    }
  }),

  reset() {
    this.set('planOutput', null);
    this.set('planError', null);
    this.set('parseError', null);
  },
});
