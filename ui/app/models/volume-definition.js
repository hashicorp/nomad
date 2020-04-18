import { alias, equal } from '@ember/object/computed';
import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  taskGroup: fragmentOwner(),

  name: attr('string'),

  source: attr('string'),
  type: attr('string'),
  readOnly: attr('boolean'),

  isCSI: equal('type', 'csi'),
  namespace: alias('taskGroup.job.namespace'),
});
