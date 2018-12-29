import { gt, alias } from '@ember/object/computed';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  deployment: fragmentOwner(),

  name: attr('string'),

  autoRevert: attr('boolean'),
  promoted: attr('boolean'),
  requiresPromotion: gt('desiredCanaries', 0),

  // The list of canary allocation IDs
  // hasMany is not supported in fragments
  placedCanaryAllocations: attr({ defaultValue: () => [] }),

  placedCanaries: alias('placedCanaryAllocations.length'),
  desiredCanaries: attr('number'),
  desiredTotal: attr('number'),
  placedAllocs: attr('number'),
  healthyAllocs: attr('number'),
  unhealthyAllocs: attr('number'),

  requireProgressBy: attr('date'),
});
