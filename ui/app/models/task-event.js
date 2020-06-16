import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class TaskEvent extends Fragment {
  @fragmentOwner() state;

  @attr('string') type;
  @attr('number') signal;
  @attr('number') exitCode;

  @attr('date') time;
  @attr('number') timeNanos;

  @attr('string') message;
}
