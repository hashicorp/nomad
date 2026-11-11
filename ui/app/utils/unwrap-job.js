/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Unwraps top-level Job wrapper if present from JSON job definitions
export default function unwrapJob(definition) {
  let unwrapped = definition;
  while (
    unwrapped &&
    typeof unwrapped === 'object' &&
    !Array.isArray(unwrapped)
  ) {
    const nested = unwrapped.Job || unwrapped.job;
    if (nested && typeof nested === 'object' && !Array.isArray(nested)) {
      unwrapped = nested;
    } else {
      break;
    }
  }
  return unwrapped;
}
