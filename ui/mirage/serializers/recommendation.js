/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['task', 'task.task-group'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(recommendationJson => serializeRecommendation(recommendationJson, this.schema));
    } else {
      serializeRecommendation(json, this.schema);
    }
    return json;
  },
});

function serializeRecommendation(recommendation, schema) {
  const taskJson = recommendation.Task;

  recommendation.Task = taskJson.Name;

  const taskGroup = schema.taskGroups.find(taskJson.TaskGroupID);

  recommendation.Group = taskGroup.name;
  recommendation.JobID = taskGroup.job.id;
  recommendation.Namespace = taskGroup.job.namespace || 'default';

  return recommendation;
}
