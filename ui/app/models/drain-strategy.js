import { lt, equal } from '@ember/object/computed';
import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import classic from 'ember-classic-decorator';

@classic
export default class DrainStrategy extends Fragment {
  @attr('number') deadline;
  @attr('date') forceDeadline;
  @attr('boolean') ignoreSystemJobs;

  @lt('deadline', 0) isForced;
  @equal('deadline', 0) hasNoDeadline;
}
