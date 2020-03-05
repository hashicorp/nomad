import Service from '@ember/service';
import config from 'nomad-ui/config/environment';

export default Service.extend({
  getTaskStateSocket(taskState, command) {
    const mirageEnabled =
      config['ember-cli-mirage'] && config['ember-cli-mirage'].enabled !== false;

    if (mirageEnabled) {
      return new Object({
        messageDisplayed: false,

        send(e) {
          if (!this.messageDisplayed) {
            this.messageDisplayed = true;
            this.onmessage({ data: `{"stdout":{"data":"${btoa('unsupported in Mirage\n\r')}"}}` });
          } else {
            this.onmessage({ data: e.replace('stdin', 'stdout') });
          }
        },
      });
    } else {
      return new WebSocket(
        // FIXME parameterise host
        `ws://localhost:4200/v1/client/allocation/${taskState.allocation.id}` +
          `/exec?task=${taskState.name}&tty=true` +
          `&command=${encodeURIComponent(`["${command}"]`)}`
      );
    }
  },
});
