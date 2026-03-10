/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    return [].concat(json).mapBy('ID');
  },
});
