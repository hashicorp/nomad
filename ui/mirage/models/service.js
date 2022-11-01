import { Model, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  job: belongsTo('job'),
});
