import { Model, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  parent: belongsTo('alloc-file'),
});
