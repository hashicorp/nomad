import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  plugin: belongsTo('csi-plugin'),
  writeAllocs: hasMany('allocation'),
  readAllocs: hasMany('allocation'),
});
