// @ts-check
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import StreamLogger from 'nomad-ui/utils/classes/stream-logger';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class RealtimeController extends Controller {
  @service token;

  @tracked stream = ['fork'];
  // @tracked stream = "foo";

  // get stream() {
  //   console.log('gettin stream');

  //   return this.model;

  //   // const stream = StreamLogger.create({
  //   //   // url: '/v1/event/stream',
  //   //   logFetch: () => this.token.authorizedRequest('/v1/event/stream'),
  //   //   params: {},
  //   // });

  //   // console.log('stream get');
  //   // stream.start();
  //   // return stream;
  //   // let reader = this.model;
  //   // console.log('reader', reader);

  //   // console.log('rez', res);
  //   // function appendToDOMStream(el) {
  //   //   return new WritableStream({
  //   //     write(chunk) {
  //   //       // el.append(chunk);
  //   //       stream.push(chunk);
  //   //       console.log('chunk', chunk);
  //   //     },
  //   //   });
  //   // }
  //   // res.body
  //   //   .pipeThrough(new TextDecoderStream())
  //   //   // .pipeThrough(upperCaseStream())
  //   //   .pipeTo(appendToDOMStream(document.body));
  //   // debugger;

  //   // return stream;

  //   return 'yeah';
  // }

  @action
  stopStream() {
    console.log('stop!');
    this.model.stop();
  }

  appendToStream(a, b, c) {
    console.log('appending', this.stream, a, b, c, this);
    let stream = this.stream;
    const context = this;
    return new WritableStream({
      write(chunk) {
        console.log('gonna write', JSON.parse(chunk), typeof chunk);
        JSON.parse(chunk).Events?.forEach((event) => stream.push(event));
        // stream.push(JSON.parse(chunk));
        context.stream = [...stream];

        console.log('chunk', chunk);
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

  @action
  startStream() {
    console.log('start!');
    console.log('stream thus', this.model);
    this.model.logFetch().then((res) => {
      console.log('res!!!', res.body);
      res.body
        .pipeThrough(new TextDecoderStream())
        .pipeThrough(this.splitStream('\n'))
        // .pipeThrough(upperCaseStream())
        .pipeTo(this.appendToStream());
    });
  }

  // @action logEvent(event) {
  //   const { Index, Topic, Type, Key, Payload, Namespace, FilterKeys } = event;
  //   console.table({
  //     Index,
  //     Topic,
  //     Type,
  //     Key,
  //     Namespace,
  //     FilterKeys,
  //   });
  //   console.log('Payload', Payload);
  //   console.log('******************************************************');
  // }
}
