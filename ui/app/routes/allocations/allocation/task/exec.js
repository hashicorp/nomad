import Route from '@ember/routing/route';

export default Route.extend({
  model() {
    const task = this.modelFor('allocations.allocation.task');
    const taskName = task.name;
    const allocationId = task.allocation.id;

    // FIXME generalise host
    const socket = new WebSocket(
      `ws://localhost:4400/v1/client/allocation/${allocationId}/exec?task=${taskName}&tty=true&command=%5B%22%2Fbin%2Fbash%22%5D`
    );

    return {
      socket,
      task,
    };
  },

  setupController(controller, { socket, task }) {
    this._super(...arguments);
    controller.setProperties({ socket, task });
  },
});
