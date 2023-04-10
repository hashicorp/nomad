/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    return [].concat(json).mapBy('ID');
  },
});
