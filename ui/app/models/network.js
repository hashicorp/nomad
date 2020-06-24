import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { array } from 'ember-data-model-fragments/attributes';

export default class Network extends Fragment {
  @attr('string') device;
  @attr('string') cidr;
  @attr('string') ip;
  @attr('string') mode;
  @attr('number') mbits;
  @array() ports;
}
