import Service from '@ember/service';

export default Service.extend({
  getTaskStateSocket(taskState, command) {
    return new WebSocket(
      `ws://localhost:4200/v1/client/allocation/${taskState.allocation.id}` +
        `/exec?task=${taskState.name}&tty=true` +
        `&command=${encodeURIComponent(`["${command}"]`)}`
    );
  },
});
