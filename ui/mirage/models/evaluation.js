import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  relatedEvals: hasMany('evaluation-stub'),
});
