// @ts-check
import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

/**
 * @typedef Event
 * @type {Object}
 * @property {number} Index
 * @property {string} Topic
 * @property {string} Type
 * @property {{string: string}[]} Key
 */

class MOCK_ABORT_CONTROLLER {
  abort() {
    /* noop */
  }
  signal = null;
}

export default class EventsService extends Service {
  @service token;

  /**
   * @type {Event[]}
   */
  @tracked
  stream = [];

  constructor() {
    super(...arguments);
    this.controller = window.AbortController
      ? new AbortController()
      : new MOCK_ABORT_CONTROLLER();
  }

  @action startFile() {
    console.log('starting file');
    this.stop(); //stops streaming in
    const reader = new FileReader();
    reader.onload = (event) => {
      const lines = event.target.result.split('\n');
      const context = this;
      context.stream = [];
      lines.forEach((line) => {
        if (line) {
          const event = JSON.parse(line);
          context.stream.push(event);
          console.log('pushed', event);
        }
      });
    };
    const read = reader.readAsText(this.file);
    console.log('read?', read);
  }

  /**
   * Starts a new event stream and populates our stream array
   */
  start() {
    console.log('Events Service starting');
    if (this.file) {
      this.startFile();
    } else {
      this.request = this.token.authorizedRequest('/v1/event/stream', {
        signal: this.controller.signal,
      });
      return this.request.then((res) => {
        res.body
          .pipeThrough(new TextDecoderStream())
          .pipeThrough(this.splitStream('\n'))
          .pipeThrough(this.parseStream())
          .pipeTo(this.appendToStream());
      });
    }
  }

  @tracked file = null;

  @action uploadFile(file) {
    console.log('upload file lol');
    this.file = file;
    this.start();
  }

  @action
  stop() {
    console.log('Events Service stopping');
    this.controller.abort();
  }

  appendToStream() {
    console.log('appendToStream()');
    let stream = this.stream;
    const context = this;
    return new WritableStream({
      write(chunk) {
        if (chunk.Events) {
          chunk.Events.forEach((event) => stream.push(event));
        }

        // Dedupe our stream by its events' "key" and "Index" fields
        context.stream = stream.reduce((acc, event) => {
          if (
            !acc.find((e) => e.Key === event.Key && e.Index === event.Index)
          ) {
            acc.push(event);
          }
          return acc;
        }, []);
      },
    });
  }

  // JSON.parses our chunks' events
  parseStream() {
    console.log('parseStream');
    return new TransformStream({
      transform(chunk, controller) {
        controller.enqueue(JSON.parse(chunk));
      },
    });
  }

  splitStream(delimiter) {
    console.log('splitStream()');
    let buffer = '';
    return new TransformStream({
      transform(chunk, controller) {
        console.log('splitStream.transform', new Date().toLocaleTimeString());
        buffer += chunk;
        let parts = buffer.split(delimiter);
        buffer = parts.pop();
        parts.forEach((p) => controller.enqueue(p));
      },
    });
  }
}
