import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';

export default class Port extends Fragment {
  @attr('string') hostIp;
  @attr('string') label;
  @attr('number') to;
  @attr('number') value;
}
