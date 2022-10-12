// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { alias } from '@ember/object/computed';
import MutableArray from '@ember/array/mutable';
import { A } from '@ember/array';
import RSVP from 'rsvp';

/**
 * @typedef {import('../services/events.js').Event} Event
 */

/**
 * @typedef Entity
 * @type {Object}
 * @property {string} id
 * @property {number} lastUpdated
 * @property {number} lastRaftIndex
 * @property {Event[]} streamEvents
 */

/**
 * @typedef Client
 * @property {MutableArray} allocations
 */

export default class EventStreamComponent extends Component {
  @service events;
  @service store;

  /**
   * @param  {[]} args
   */
  constructor(...args) {
    super(...args);
    this.events.start();
  }

  /**
   * @type {MutableArray<Client & Entity>}
   */
  @tracked clients = A([]);

  /**
   * @type {MutableArray<Entity>}
   */
  @tracked servers = A([]);

  /**
   * @type {MutableArray<Entity>}
   */
  @tracked allocations = A([]);

  /**
   * @type {MutableArray<Entity>}
   */
  @tracked jobs = A([]);

  @computed('stream.[]', 'clients', 'servers', 'allocations', 'jobs')
  get entities() {
    const { clients, servers, allocations, jobs } = this;
    if (clients.length) {
      this.amendClients(clients);
    }
    return { clients, servers, allocations, jobs };
  }

  @action
  async fetchEntities() {
    const entities = RSVP.hash({
      jobs: this.store.findAll('job'),
      allocations: this.store.findAll('allocation'),
      clients: this.store.findAll('node'),
      servers: this.store.findAll('agent'),
    })
      .catch((error) => {
        console.log('Error fetching entities', error);
      })
      .then((entities) => {
        this.jobs = entities.jobs;
        this.allocations = entities.allocations;
        this.clients = entities.clients;
        this.servers = entities.servers;
      });
  }

  @alias('events.stream') stream;

  get streamIndexes() {
    return this.stream.mapBy('Index').uniq();
  }

  eventsNotInEntities = {
    clients: [],
    servers: [],
    allocations: [],
    jobs: [],
  };

  //#region Entity Processing

  /**
   *
   * @param {MutableArray<Client & Entity>} clients
   */
  amendClients(clients) {
    clients.forEach((client) => {
      client.lastUpdated = 150;

      /**
       * @type {MutableArray<Event>}
       */
      const clientAllocationEvents = this.stream.filterBy(
        'Payload.Allocation.NodeID',
        client.id
      );

      // Sometimes an event stream may have an event for an allocation that's been otherwise garbage-collected
      // Let's not try to reload over and over again when we come across this. Save a record of it to a hash and ignore it.
      const eventedAllocationsNotInClient = clientAllocationEvents
        .uniqBy('Key')
        .reject((event) =>
          this.eventsNotInEntities.allocations.includes(event.Key)
        )
        .filter((event) => !client.allocations.mapBy('id').includes(event.Key))
        .mapBy('Key');

      if (eventedAllocationsNotInClient.length) {
        this.eventsNotInEntities.allocations.push(
          ...eventedAllocationsNotInClient
        );
        this.fetchEntities(); // TODO: change to fetchClients?
        return false;
      }

      // If an alloc event indicates a different clientStatus than the client.allocation's status, amend the client's allocation's status
      const lastAllocStatuses = clientAllocationEvents
        .sortBy('Index')

        .reverse()
        .uniqBy('Key');
      lastAllocStatuses.forEach((event) => {
        const alloc = client.allocations.findBy('id', event.Key);
        if (
          alloc &&
          alloc.clientStatus !== event.Payload.Allocation.ClientStatus
        ) {
          alloc.clientStatus = event.Payload.Allocation.ClientStatus;
        }
      });

      client.streamEvents = [...clientAllocationEvents];
      client.lastUpdated =
        client.streamEvents.lastObject?.Payload.Allocation.ModifyTime / 1000000;
      client.lastRaftIndex = client.streamEvents.lastObject?.Index;
    });
  }

  //#endregion Entity Processing

  scrollToEnd(element) {
    element.scrollLeft = element.scrollWidth;
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
