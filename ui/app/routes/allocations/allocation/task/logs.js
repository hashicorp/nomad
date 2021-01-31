import Route from '@ember/routing/route';

export default class LogsRoute extends Route {
  model() {
    const task = super.model(...arguments);
    return task && task.get('allocation.node').then(() => task);
  }
}
