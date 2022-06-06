import Route from '@ember/routing/route';

export default class VariablesNewRoute extends Route {
  model(params) {
    return this.store.createRecord('variable', { path: params.path });
  }
  resetController(controller, isExiting) {
    if (isExiting) {
      // If user didn't save, delete the freshly created model
      if (controller.model.isNew) {
        controller.model.destroyRecord();
      }
    }
  }
}
