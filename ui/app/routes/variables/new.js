import Route from '@ember/routing/route';

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
