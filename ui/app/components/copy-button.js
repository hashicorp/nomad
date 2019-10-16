import Component from '@ember/component';
import { task, timeout } from 'ember-concurrency';

/**
 * @module CopyButton
 * The `CopyButton` shows a button that copies a string to the clipboard upon click. It shows a temporary tooltip upon success or a permanent one upon failure.
 *
 * @example
 * {{copy-button clipboardText='the text to copy'}}```
 *
 * @param clipboardText {String} - text to copy to the clipboard
 *
 */
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
