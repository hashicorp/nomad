import Component from '@ember/component';
import { task, timeout } from 'ember-concurrency';

export default Component.extend({
  classNames: ['copy-button'],

  clipboardText: null,
  state: null,

  indicateSuccess: task(function*() {
    this.set('state', 'success');

    yield timeout(2000);
    this.set('state', null);
  }).restartable(),
});
