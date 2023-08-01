import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  events: hasMany('node-event'),
});
