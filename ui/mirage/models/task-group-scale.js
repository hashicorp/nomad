import { Model, belongsTo, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  jobScale: belongsTo(),
  events: hasMany('scale-event'),
});
