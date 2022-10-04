// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class EventStreamComponent extends Component {
  @service events;

  constructor() {
    super(...arguments);
    this.events.start();
  }

  get stream() {
    // console.log('gettin stream', this.events.stream);
    return this.events.stream;
  }

  @action logEvent(event) {
    console.clear();
    const { Index, Topic, Type, Key, Payload, Namespace, FilterKeys } = event;
    console.table({
      Index,
      Topic,
      Type,
      Key,
      Namespace,
      // FilterKeys,
    });
    console.log('Payload');
    console.log(Payload[Topic]);
    console.log('Same-Time Buds');
    console.table(this.stream.filterBy('Index', Index));
    console.log('Same-Entity Buds');
    console.table(this.stream.filterBy('Key', Key));
    console.log('******************************************************');
  }
}
