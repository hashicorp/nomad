import Route from '@ember/routing/route';
// import fetch from 'nomad-ui/utils/fetch';
import { inject as service } from '@ember/service';
import StreamLogger from 'nomad-ui/utils/classes/stream-logger';

export default class RealtimeRoute extends Route {
  @service token;
  get secret() {
    return window.localStorage.nomadTokenSecret;
  }

  async model() {
    // const data = await fetch('/v1/event/stream', {});
    // const streamResponse = this.token.authorizedRequest(
    //   '/v1/event/stream',
    //   {
    //     method: 'GET',
    //   }
    // );

    const stream = StreamLogger.create({
      url: '/v1/event/stream',
      logFetch: (url) => this.token.authorizedRequest('/v1/event/stream'),
      params: {},
    });

    console.log('tokin', this.token);
    console.log('yeah ok', stream);
    stream.start();
    return stream;
  }
}
