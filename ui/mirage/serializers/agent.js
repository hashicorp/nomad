/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
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
