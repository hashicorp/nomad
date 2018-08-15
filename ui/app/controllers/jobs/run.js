import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { task } from 'ember-concurrency';

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
      console.log('Heyo!', planOutput);
      this.set('planOutput', planOutput);
    } catch (err) {
      this.set('planError', err);
      console.log('Uhoh', err);
    }
  }).drop(),

  submit: task(function*() {}),

  cancel() {
    this.set('planOutput', null);
    this.set('planError', null);
    this.set('parseError', null);
  },
});
