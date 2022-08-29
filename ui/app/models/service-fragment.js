import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';

export default class Service extends Fragment {
  @attr('string') name;
  @attr('string') portLabel;
  @attr() tags;
  @attr('string') onUpdate;
  @attr('string') provider;
  @fragment('consul-connect') connect;
  @attr() groupName;
  @attr() taskName;

  get refID() {
    return `${this.groupName || this.taskName}-${this.name}`;
  }
}
