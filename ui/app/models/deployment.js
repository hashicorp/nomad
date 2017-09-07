import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default Model.extend({
  job: belongsTo('job'),
  version: attr('number'),
  status: attr('string'),
  statusDescription: attr('string'),
  taskGroupSummaries: fragmentArray('task-group-deployment-summary'),
});
