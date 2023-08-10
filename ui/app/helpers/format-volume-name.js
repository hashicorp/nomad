/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

/**
 * Volume Name Formatter
 *
 * Usage: {{format-volume-name source=string isPerAlloc=boolean volumeExtension=string}}
 *
 * Outputs a title/link for volumes that are per_alloc-aware.
 * (when a volume is per_alloc, its route location requires an additional extension)
 */
export function formatVolumeName(
  _,
  { source = '', isPerAlloc, volumeExtension }
) {
  return `${source}${isPerAlloc ? volumeExtension : ''}`;
}

export default helper(formatVolumeName);
