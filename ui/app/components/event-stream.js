// @ts-check
import Component from '@glimmer/component';
import { logger } from 'nomad-ui/utils/classes/log';
import RSVP from 'rsvp';
import timeout from 'nomad-ui/utils/timeout';

class MockAbortController {
  abort() {
    /* noop */
  }
  signal = null;
}

export default class EventStreamComponent extends Component {
  // clientTimeout = 1000;
  // get logUrl() {
  //   return 'v1/event/stream';
  // }
  // get logParams() {
  //   return {};
  // }
  // @logger(this.logUrl, this.logParams, function logFetch() {
  //   const aborter = window.AbortController
  //     ? new AbortController()
  //     : new MockAbortController();
  //   // Capture the state of useServer at logger create time to avoid a race
  //   // between the stdout logger and stderr logger running at once.
  //   return (url) =>
  //     RSVP.race([
  //       this.token.authorizedRequest(url, { signal: aborter.signal }),
  //       timeout(this.clientTimeout),
  //     ]).then(
  //       (response) => {
  //         return response;
  //       },
  //       (error) => {
  //         aborter.abort();
  //         this.set('noConnection', true);
  //         throw error;
  //       }
  //     );
  // })
  // logger;
}
