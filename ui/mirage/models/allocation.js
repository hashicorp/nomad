import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  task_states: hasMany('task-state'),
  task_resources: hasMany('task-resource'),
});
