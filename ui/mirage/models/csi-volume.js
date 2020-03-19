import { Model, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  plugin: belongsTo('csi-plugin'),
});
