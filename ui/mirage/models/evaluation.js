import { Model, hasMany, belongsTo } from 'ember-cli-mirage';

export default Model.extend({
  relatedEvals: hasMany('evaluation-stub'),
});
