import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default Route.extend({
  store: service(),
  system: service(),

  breadcrumbs: [
    {
      label: 'Run',
      args: ['jobs.run'],
    },
  ],

  model() {
    return this.store.createRecord('job', {
      namespace: this.get('system.activeNamespace'),
    });
  },

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.model.deleteRecord();
    }
  },
});
