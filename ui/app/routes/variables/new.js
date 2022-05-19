import Route from '@ember/routing/route';
import { action } from '@ember/object';

export default class VariablesNewRoute extends Route {
  model() {
    return this.store.findAll('namespace').then(() => {
      return this.store.createRecord('var');
    });
  }
  resetController(controller, isExiting) {
    if (isExiting) {
      controller.model.deleteRecord();
    }
  }
}
