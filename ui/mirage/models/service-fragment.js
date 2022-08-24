import { Model, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  taskGroup: belongsTo('task-group'),
});
