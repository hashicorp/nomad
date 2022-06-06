import Route from '@ember/routing/route';

export default class VariablesNewRoute extends Route {
  model(params) {
    return this.store.createRecord('variable', { path: params.path });
  }
  resetController(controller, isExiting) {
    // If the user navigates away from /new, clear the path
    controller.set('path', null);
    if (isExiting) {
      // If user didn't save, delete the freshly created model
      if (controller.model.isNew) {
        controller.model.destroyRecord();
      }
    }
  }
}
