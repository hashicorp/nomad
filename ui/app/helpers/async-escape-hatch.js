/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export function asyncEscapeHatch([model, relationship]) {
  return model[relationship].content;
}

export default Helper.helper(asyncEscapeHatch);
