/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

// Generated at compile-time by ember-inline-svg
import SVGs from '../svgs';

/**
 * Icons array
 *
 * Usage: {{#each (all-icons) as |icon|}}
 *
 * Returns the array of all icon strings available to {{x-icon}}. This is a bit of a hack
 * since the above SVGs import isn't available in the Storybook context.
 */
export function allIcons() {
  return Object.keys(SVGs);
}

export default helper(allIcons);
