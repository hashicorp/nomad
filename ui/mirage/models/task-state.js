import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  allocation: belongsTo(),
  events: hasMany('task-event'),
});
