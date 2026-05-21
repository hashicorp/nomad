/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export function formatID([model, relationship]) {
  // store.unloadRecord() does not set isDestroyed=true on the Ember Object, so
  // we cannot use that flag as a guard. Use try/catch instead: if the record
  // has been removed from the store its internal identifier is gone and
  // belongsTo() will throw an assertion error.
  try {
    const id = model.belongsTo(relationship).id();
    return { id, shortId: id?.split('-')[0] };
  } catch {
    return { id: null, shortId: null };
  }
}

export default Helper.helper(formatID);
