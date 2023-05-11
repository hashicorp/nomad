/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';

export function formatID([model, relationship]) {
  const id = model.belongsTo(relationship).id();
  return { id, shortId: id.split('-')[0] };
}

export default Helper.helper(formatID);
