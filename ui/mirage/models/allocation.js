import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  task_states: hasMany('task-state'),
});
