/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export function asyncEscapeHatch([model, relationship]) {
  // Guard against accessing async relationship proxies on records that have
  // been unloaded from the store (e.g. after removeRecord). The optional chain
  // on the relationship handles the null-belongsTo case set by removeRecord,
  // and the try/catch handles the fully-unloaded record case.
  try {
    return model[relationship]?.content ?? null;
  } catch {
    return null;
  }
}

export default Helper.helper(asyncEscapeHatch);
