import Service from '@ember/service';
import config from 'nomad-ui/config/environment';
import { getOwner } from '@ember/application';

export default class SocketsService extends Service {
  getTaskStateSocket(taskState, command) {
    const mirageEnabled =
      config.environment !== 'production' &&
      config['ember-cli-mirage'] &&
      config['ember-cli-mirage'].enabled !== false;

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
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const applicationAdapter = getOwner(this).lookup('adapter:application');
      const prefix = `${applicationAdapter.host ||
        window.location.host}/${applicationAdapter.urlPrefix()}`;

      return new WebSocket(
        `${protocol}//${prefix}/client/allocation/${taskState.allocation.id}` +
          `/exec?task=${taskState.name}&tty=true&ws_handshake=true` +
          `&command=${encodeURIComponent(`["${command}"]`)}`
      );
    }
  }
}
