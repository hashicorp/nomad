import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';

export default class PlacementFailure extends Fragment {
  @attr('string') name;

  @attr('number') coalescedFailures;

  @attr('number') nodesEvaluated;
  @attr('number') nodesExhausted;

  // Maps keyed by relevant dimension (dc, class, constraint, etc)ith count values
  @attr() nodesAvailable;
  @attr() classFiltered;
  @attr() constraintFiltered;
  @attr() classExhausted;
  @attr() dimensionExhausted;
  @attr() quotaExhausted;
  @attr() scores;
}
