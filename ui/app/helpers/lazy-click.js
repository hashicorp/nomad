import Helper from '@ember/component/helper';
import $ from 'jquery';

/**
 * Lazy Click Event
 *
 * Usage: {{lazy-click action}}
 *
 * Calls the provided action only if the target isn't an anchor
 * that should be handled instead.
 */
export function lazyClick([onClick, event]) {
  if (!$(event.target).is('a')) {
    onClick(event);
  }
}

export default Helper.helper(lazyClick);
