import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  deploymentTaskGroupSummaries: hasMany('deployment-task-group-summary'),
});
