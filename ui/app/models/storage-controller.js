import { attr, belongsTo } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class StorageController extends Fragment {
  @fragmentOwner() plugin;

  @belongsTo('node') node;
  @attr('string') allocID;

  @attr('string') provider;
  @attr('string') version;
  @attr('boolean') healthy;
  @attr('string') healthDescription;
  @attr('date') updateTime;
  @attr('boolean') requiresControllerPlugin;
  @attr('boolean') requiresTopologies;

  @attr() controllerInfo;

  // Fragments can't have relationships, so provider a manual getter instead.
  async getAllocation() {
    return this.store.findRecord('allocation', this.allocID);
  }
}
