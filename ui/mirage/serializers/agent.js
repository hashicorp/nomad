/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  keyForAttribute(str) {
    if (str === 'config' || str === 'member') {
      return str;
    }
    return ApplicationSerializer.prototype.keyForAttribute.apply(this, arguments);
  },
});
