import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  taskGroup: belongsTo(),
  recommendations: hasMany(),
});
