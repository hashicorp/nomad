import { Model, hasMany, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  taskGroups: hasMany('task-group'),
  jobSummary: belongsTo('job-summary'),
  jobScale: belongsTo('job-scale'),
});
