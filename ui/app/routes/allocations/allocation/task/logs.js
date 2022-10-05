import Route from '@ember/routing/route';

export default class LogsRoute extends Route {
  async model() {
    const task = super.model(...arguments);

    if (task) {
      await task.get('allocation.node');
      return task;
    }
  }
}
