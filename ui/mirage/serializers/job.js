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
  job.TaskGroups.forEach((group) => {
    if (group.Services.length === 0) {
      group.Services = null;
    }
    if (group.Tasks) {
      group.Tasks = group.Tasks.map((task) => serializeTaskFragment(task));
    }
  });
}

function serializeTaskFragment(task) {
  return {
    ...task,
    actions: task.Actions.map(serializeActionFragment),
  };
}

function serializeActionFragment(action) {
  return {
    name: action.Name,
    command: action.Command,
    args: action.Args,
  };
}
