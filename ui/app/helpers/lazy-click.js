/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

/**
 * Lazy Click Event
 *
 * Usage: {{lazy-click action}}
 *
 * Calls the provided action only if the target isn't an anchor
 * that should be handled instead.
 */
export function lazyClick([onClick, event]) {
  if (!['a', 'button'].includes(event?.target.tagName.toLowerCase())) {
    onClick(event);
  }
}

export default Helper.helper(lazyClick);
