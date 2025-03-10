/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['allocations', 'node'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (!Array.isArray(json)) {
      serializeVolume(json);
    }
    return json;
  },
});

function serializeVolume(volume) {
  volume.NodeID = volume.Node.ID;
  delete volume.Node;
}
