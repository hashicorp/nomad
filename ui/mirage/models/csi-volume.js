import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  plugin: belongsTo('csi-plugin'),
  allocations: hasMany('allocation'),
});
