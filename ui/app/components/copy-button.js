import Component from '@ember/component';
import { run } from '@ember/runloop';
import { task } from 'ember-concurrency';

export default Component.extend({
  classNames: ['copy-button'],

  clipboardText: null,
  state: null,

  indicateSuccess: task(function*() {
    this.set('state', 'success');

    yield run.later(() => {
      this.set('state', null);
    }, 2000);
  }),
});
