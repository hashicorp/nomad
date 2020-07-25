import { Model, hasMany, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  task_groups: hasMany('task-group'),
  job_summary: belongsTo('job-summary'),
  job_scale: belongsTo('job-scale'),
});
