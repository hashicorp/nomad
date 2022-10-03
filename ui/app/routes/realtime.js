import Route from '@ember/routing/route';
// import fetch from 'nomad-ui/utils/fetch';
import { inject as service } from '@ember/service';
import StreamLogger from 'nomad-ui/utils/classes/stream-logger';

class MockAbortController {
  abort() {
    /* noop */
  }
  signal = null;
}
export default class RealtimeRoute extends Route {
  @service token;
  get secret() {
    return window.localStorage.nomadTokenSecret;
  }

  async model() {
    const aborter = window.AbortController
      ? new AbortController()
      : new MockAbortController();

    const stream = StreamLogger.create({
      // url: '/v1/event/stream',
      logFetch: () =>
        this.token.authorizedRequest('/v1/event/stream', {
          signal: aborter.signal,
        }),
      params: {},
    });

    console.log('tokin', this.token);
    console.log('yeah ok', stream);
    return stream;
  }
}
