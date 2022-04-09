import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';

export default class Port extends Fragment {
  @attr('string') hostIp;
  @attr('string') label;
  @attr('number') to;
  @attr('number') value;
}
