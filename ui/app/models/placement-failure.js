import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';

export default Fragment.extend({
  name: attr('string'),

  coalescedFailures: attr('number'),

  nodesEvaluated: attr('number'),
  nodesExhausted: attr('number'),

  // Maps keyed by relevant dimension (dc, class, constraint, etc) with count values
  nodesAvailable: attr(),
  classFiltered: attr(),
  constraintFiltered: attr(),
  classExhausted: attr(),
  dimensionExhausted: attr(),
  quotaExhausted: attr(),
  scores: attr(),
});
