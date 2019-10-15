import Component from '@ember/component';
import { task, timeout } from 'ember-concurrency';

/**
 * @module CopyButton
 * The `CopyButton` is SOMETHING
 *
 * @example ```js
 * // a comment?
 *   {{copy-button clipboardText='the text ya'}}```
 *
 * @param [clipboardText=null] {String} - The text to copy to the clipboard.
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
