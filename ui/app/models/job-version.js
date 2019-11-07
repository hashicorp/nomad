import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';

export default Model.extend({
  job: belongsTo('job'),
  stable: attr('boolean'),
  submitTime: attr('date'),
  number: attr('number'),
  diff: attr(),
});
