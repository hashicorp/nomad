/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';

export function asyncEscapeHatch([model, relationship]) {
  return model[relationship].content;
}

export default Helper.helper(asyncEscapeHatch);
