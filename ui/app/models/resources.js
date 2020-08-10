import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default class Resources extends Fragment {
  @attr('number') cpu;
  @attr('number') memory;
  @attr('number') disk;
  @attr('number') iops;
  @fragmentArray('network', { defaultValue: () => [] }) networks;
  @fragmentArray('port', { defaultValue: () => [] }) ports;
}
