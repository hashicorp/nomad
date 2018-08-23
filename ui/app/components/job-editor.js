import Component from '@ember/component';
import { assert } from '@ember/debug';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { task } from 'ember-concurrency';
import messageFromAdapterError from 'nomad-ui/utils/message-from-adapter-error';
import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

export default Component.extend({
  store: service(),
  config: service(),

  'data-test-job-editor': true,

  job: null,
  onSubmit() {},
  context: computed({
    get() {
      return this.get('_context');
    },
    set(key, value) {
      const allowedValues = ['new', 'edit'];

      assert(`context must be one of: ${allowedValues.join(', ')}`, allowedValues.includes(value));

      this.set('_context', value);
      return value;
    },
  }),

  _context: null,
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
      yield this.get('job').parse();
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not parse input';
      this.set('parseError', error);
      this.scrollToError();
      return;
    }

    try {
      const plan = yield this.get('job').plan();
      this.set('planOutput', plan);
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not plan job';
      this.set('planError', error);
      this.scrollToError();
    }
  }).drop(),

  submit: task(function*() {
    try {
      if (this.get('context') === 'new') {
        yield this.get('job').run();
      } else {
        yield this.get('job').update();
      }

      const id = this.get('job.plainId');
      const namespace = this.get('job.namespace.name') || 'default';

      this.reset();

      // Treat the job as ephemeral and only provide ID parts.
      this.get('onSubmit')(id, namespace);
    } catch (err) {
      const error = messageFromAdapterError(err) || 'Could not submit job';
      this.set('runError', error);
      this.set('planOutput', null);
      this.scrollToError();
    }
  }),

  reset() {
    this.set('planOutput', null);
    this.set('planError', null);
    this.set('parseError', null);
    this.set('runError', null);
  },

  scrollToError() {
    if (!this.get('config.isTest')) {
      window.scrollTo(0, 0);
    }
  },
});
