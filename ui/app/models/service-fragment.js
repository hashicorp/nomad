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

  get fragOwner() {
    return this._internalModel._recordData._owner;
  }

  get isTaskLevel() {
    return this.fragOwner._internalModel.modelName === 'task';
  }
}
