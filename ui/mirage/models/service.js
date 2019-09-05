import { Model, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  task_group: belongsTo('task-group'),
});
