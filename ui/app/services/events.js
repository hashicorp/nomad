// @ts-check
import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

class MOCK_ABORT_CONTROLLER {
  abort() {
    /* noop */
  }
  signal = null;
}

export default class EventsService extends Service {
  @service token;

  @tracked
  stream = [];

  constructor() {
    super(...arguments);
    this.controller = window.AbortController
      ? new AbortController()
      : new MOCK_ABORT_CONTROLLER();
  }

  /**
   * Starts a new event stream and populates our stream array
   */
  start() {
    console.log('Events Service starting');
    this.request = this.token.authorizedRequest('/v1/event/stream', {
      signal: this.controller.signal,
    });
    return this.request.then((res) => {
      res.body
        .pipeThrough(new TextDecoderStream())
        .pipeThrough(this.splitStream('\n'))
        // .pipeThrough(upperCaseStream())
        .pipeTo(this.appendToStream());
    });
  }

  @action
  stop() {
    console.log('Events Service stopping');
    this.controller.abort();
  }

  appendToStream() {
    // console.log('appending', this.stream);
    let stream = this.stream;
    const context = this;
    return new WritableStream({
      write(chunk) {
        // console.log('gonna write', JSON.parse(chunk), typeof chunk);
        JSON.parse(chunk).Events?.forEach((event) => stream.push(event));
        context.stream = [...stream];
        // console.log('afterwards', context.stream);

        // console.log('chunk', chunk);
      },
    });
  }

  //
  splitStream(delimiter) {
    let buffer = '';
    return new TransformStream({
      transform(chunk, controller) {
        buffer += chunk;
        let parts = buffer.split(delimiter);
        buffer = parts.pop();
        parts.forEach((p) => controller.enqueue(p));
      },
    });
  }
}
