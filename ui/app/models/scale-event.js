import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class ScaleEvent extends Fragment {
  @fragmentOwner() taskGroupScale;

  @attr('number') count;
  @attr('number') previousCount;
  @attr('boolean') error;
  @attr('string') evalId;

  @attr('date') time;
  @attr('number') timeNanos;

  @attr('string') message;
  @attr() meta;
}
