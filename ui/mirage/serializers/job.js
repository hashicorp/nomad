/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['taskGroups', 'jobSummary'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeJob);
    } else {
      serializeJob(json);
    }
    return json;
  },
});

function serializeJob(job) {
  job.TaskGroups.forEach(group => {
    if (group.Services.length === 0) {
      group.Services = null;
    }
  });
}
