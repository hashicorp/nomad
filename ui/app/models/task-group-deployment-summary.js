import Ember from 'ember';
import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

const { computed } = Ember;

export default Fragment.extend({
  deployment: fragmentOwner(),

  name: attr('string'),

  autoRevert: attr('boolean'),
  promoted: attr('boolean'),
  requiresPromotion: computed.gt('desiredCanaries', 0),

  placedCanaries: attr('number'),
  desiredCanaries: attr('number'),
  desiredTotal: attr('number'),
  placedAllocs: attr('number'),
  healthyAllocs: attr('number'),
  unhealthyAllocs: attr('number'),
});
