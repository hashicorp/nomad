import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';

export default class ServiceFragment extends Fragment {
  @attr('string') name;
  @attr('string') portLabel;
  @attr() tags;
  @attr('string') onUpdate;
  @fragment('consul-connect') connect;
}
