import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import classic from 'ember-classic-decorator';

@classic
export default class TaskGroup extends Fragment {
  @fragmentOwner() taskGroup;

  @attr('boolean') enabled;
  @attr('number') max;
  @attr('number') min;

  @attr() policy;
}
