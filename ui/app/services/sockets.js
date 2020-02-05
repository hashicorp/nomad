import Service from '@ember/service';

export default Service.extend({
  getTaskStateSocket(taskState) {
    return new WebSocket(
      `ws://localhost:4200/v1/client/allocation/${taskState.allocation.id}/exec?task=${taskState.name}&tty=true&command=%5B%22%2Fbin%2Fbash%22%5D`
    );
  },
});
