// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class EventStreamComponent extends Component {
  @service events;
  @service store;

  constructor() {
    super(...arguments);
    this.events.start();
  }

  @tracked clients = [];
  @tracked servers = [];
  @tracked allocations = [];
  @tracked jobs = [];

  get entities() {
    const { clients, servers, allocations, jobs } = this;
    return { clients, servers, allocations, jobs };
  }

  @action
  async fetchEntities() {
    this.clients = await this.store.findAll('node');
    this.servers = await this.store.findAll('agent');
    this.allocations = await this.store.findAll('allocation');
    this.jobs = await this.store.findAll('job');
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
