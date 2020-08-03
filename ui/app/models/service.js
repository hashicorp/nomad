import attr from 'ember-data/attr';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';

export default class Service extends Fragment {
  @attr('string') name;
  @attr('string') portLabel;
  @attr() tags;
  @fragment('consul-connect') connect;
}
