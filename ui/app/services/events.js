// @ts-check
import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

/**
 * @typedef StreamEvent
 * @type {Object}
 * @property {number} Index
 * @property {string} Topic
 * @property {Object} Payload
 */

/**
 * @typedef Event
 * @type {Object}
 * @property {number} streamEventIndex
 * @property {string} streamEventTopic
 * @property {string} jobName
 * @property {string} taskName
 *
 */

/**
 * @typedef TaskEvent
 * @type {Object}
 * @property {string} DisplayMessage
 * @property {string} Type
 * @property {string} Time // maybe?
 */

/**
 * @typedef NodeEvent
 * @type {Object}
 * @property {string} Message
 * @property {string} Timestamp
 *
 */

/**
 * @typedef EventSubscription
 * @type {Object}
 * @property {"Allocation" | "Evaluation" |"Node"} Topic
 * @property {EventSubscriptionCondition[]} conditions
 * @property {boolean} playSound
 * @property {string} notificationType
 * @property {boolean} [muted]
 */

/**
 * @typedef EventSubscriptionCondition
 * @type {AllocationEventSubscriptionCondition | NodeEventSubscriptionCondition}
 */

class MOCK_ABORT_CONTROLLER {
  abort() {
    /* noop */
  }
  signal = null;
}

export default class EventsService extends Service {
  @service token;
  @service notifications;

  @tracked sidebarIsActive = false;

  /**
   * @type {Event[]}
   */
  @tracked stream = [];

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

    // TODO: World's hackiest nonsense. Get the latest index seen after 1 second of waiting.
    setTimeout(() => {
      this.observationStartIndex =
        this.stream[this.stream.length - 1]?.streamEventIndex;
      console.log('observationStartIndex', this.observationStartIndex);
    }, 1000);

    this.request = this.token.authorizedRequest('/v1/event/stream', {
      signal: this.controller.signal,
    });
    return this.request.then((res) => {
      console.log('resres', res.body);
      res.body
        .pipeThrough(new TextDecoderStream())
        .pipeThrough(this.splitStream('\n'))
        .pipeThrough(this.parseStream())
        .pipeThrough(this.extractEvents())
        .pipeThrough(this.checkForSubscription())
        .pipeTo(this.appendToStream());
    });
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
        stream.push(chunk);
        //   if (chunk.Events) {
        //     chunk.Events.forEach((event) => stream.push(event));
        //   }

        //   // Dedupe our stream by its events' "key" and "Index" fields
        //   context.stream = stream.reduce((acc, event) => {
        //     if (
        //       !acc.find((e) => e.Key === event.Key && e.Index === event.Index)
        //     ) {
        //       acc.push(event);
        //     }
        //     return acc;
        //   }, []);
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
        buffer += chunk;
        let parts = buffer.split(delimiter);
        buffer = parts.pop();
        parts.forEach((p) => controller.enqueue(p));
      },
    });
  }

  //#region Subscriptions

  /**
   * Because all Event Stream events contain a payload that may have PREVIOUS events of the entity (say, Allocation's Task) in question,
   * we need to extract those events and add them to our stream for deduping and subscription matching.
   * @returns {TransformStream}
   */

  @tracked allEvents = new Set();
  extractEvents() {
    console.log('extractEvents()');
    const context = this;
    return new TransformStream({
      // TODO: THURSDAY: ACTUALLY MAKE THIS WORK
      transform(chunk, controller) {
        if (chunk.Events) {
          chunk.Events.forEach((event) => {
            if (event.Payload) {
              if (event.Payload.Allocation) {
                if (event.Payload.Allocation.TaskStates) {
                  Object.entries(event.Payload.Allocation.TaskStates).forEach(
                    ([taskName, taskState]) => {
                      if (taskState.Events) {
                        taskState.Events.forEach((taskEvent) => {
                          // Create a key for de-duplication
                          const key = context.createKey(event, taskEvent);

                          // Check if this event has been seen before
                          if (!context.allEvents.has(key)) {
                            // Create a new chunk for each task event and enqueue it
                            /**
                             * @type {Event}
                             */
                            const newEvent = {
                              streamEventTopic: event.Topic,
                              streamEventIndex: event.Index,
                              jobName: event.Payload.Allocation.JobID,
                              taskName,
                              ...taskEvent,
                            };
                            controller.enqueue(newEvent);

                            // Mark the event as seen
                            context.allEvents.add(key);
                          }
                        });
                      }
                    }
                  );
                }
              } else if (event.Payload.Node) {
                console.log('no-devent', event);
                if (event.Payload.Node.Events) {
                  event.Payload.Node.Events.forEach((nodeEvent) => {
                    // Create a key for de-duplication
                    const key = context.createNodeEventKey(event, nodeEvent);
                    console.log('key is', key);

                    // Check if this event has been seen before
                    if (!context.allEvents.has(key)) {
                      // Create a new chunk for each task event and enqueue it
                      /**
                       * @type {Event}
                       */
                      const newEvent = {
                        streamEventTopic: event.Topic,
                        streamEventIndex: event.Index,
                        nodeName: event.Payload.Node.Name,
                        ...nodeEvent,
                      };
                      controller.enqueue(newEvent);

                      // Mark the event as seen
                      context.allEvents.add(key);
                    }
                  });
                }
              }
            }
          });
        }
        // controller.enqueue(chunk); // Enqueue the chunk unchanged
      },
    });
  }

  // TODO: this really only works for task events. We need to generalize this to all events.
  /**
   *
   * @param {StreamEvent} event
   * @param {TaskEvent} taskEvent
   * @returns {string}
   */
  createKey(event, taskEvent) {
    // Combine relevant fields to create a unique key for the event
    return `${event.Topic}-${taskEvent.Time}-${taskEvent.Type}`;
  }

  /**
   * @param {StreamEvent} event
   * @param {NodeEvent} nodeEvent
   */
  createNodeEventKey(event, nodeEvent) {
    return `${event.Topic}-${nodeEvent.Timestamp}-${nodeEvent.Message}`;
  }

  /**
   * The time before which we don't care about event notifications
   */
  @tracked observationStartIndex = 1; // TODO: make null

  /**
   * @typedef AllocationEventSubscriptionCondition
   * @type {Object}
   * @property {"DisplayMessage" | "Type"} stringKey
   * @property {string[]} tasks - The tasks to match on. ["*"] matches all tasks.
   * @property {string[]} jobs - The jobs to match on. ["*"] matches all jobs.
   * @property {"equals" | "contains"} matchType
   * @property {string} value
   */

  /**
   * @typedef NodeEventSubscriptionCondition
   * @type {Object}
   * @property {"Message"} stringKey
   * @property {"equals" | "contains"} matchType
   * @property {string} value
   * @property {string[]} nodes - The jobs to match on. ["*"] matches all jobs.
   */

  /**
   * @type {EventSubscription[]}
   */
  // @tracked subscriptions = [];
  @tracked subscriptions = [
    // {
    //   Topic: "Allocation",
    //   Type: "AllocationUpdated",
    //   playSound: false,
    //   conditions: [
    //     // {
    //     //   stringKey: "DisplayMessage",
    //     //   tasks: ["roll dice"], // ["*"] etc.
    //     //   jobs: ["fails_every_10"], // ["*"] etc.
    //     //   matchType: "contains", // "equals", "contains"
    //     //   value: "Task started by client",
    //     // }
    //     {
    //       stringKey: "DisplayMessage",
    //       tasks: ["*"], // ["*"] etc.
    //       jobs: ["*"], // ["*"] etc.
    //       matchType: "contains", // "equals", "contains"
    //       value: "Building Task Directory",
    //     }
    //   ]
    // },
    {
      Topic: 'Allocation',
      playSound: true,
      notificationType: 'critical',
      muted: true,
      conditions: [
        {
          stringKey: 'Type',
          tasks: ['*'], // ["*"] etc.
          jobs: ['*'], // ["*"] etc.
          matchType: 'equals', // "equals", "contains"
          value: 'Not Restarting',
        },
      ],
    },
    {
      Topic: 'Allocation',
      playSound: true,
      notificationType: 'warning',
      conditions: [
        {
          stringKey: 'Type',
          tasks: ['*'], // ["*"] etc.
          jobs: ['*'], // ["*"] etc.
          matchType: 'equals', // "equals", "contains"
          value: 'Restarting',
        },
      ],
    },
    {
      Topic: 'Node',
      playSound: true,
      notificationType: 'critical',
      conditions: [
        {
          stringKey: 'Message',
          matchType: 'equals',
          value: 'Node heartbeat missed',
          nodes: ['*'],
        },
      ],
    },
    {
      Topic: 'Node',
      playSound: true,
      notificationType: 'success',
      conditions: [
        {
          stringKey: 'Message',
          matchType: 'contains',
          value: 'Node registered',
          nodes: ['*'],
        },
      ],
    },
    {
      Topic: 'Node',
      playSound: true,
      notificationType: 'success',
      conditions: [
        {
          stringKey: 'Message',
          matchType: 'contains',
          value: 'Node reregistered',
          nodes: ['*'],
        },
      ],
    },
  ];

  /**
   * Checks stream events to see if they match subscription, and if so, fire a notification
   * @returns {TransformStream}
   **/
  checkForSubscription() {
    console.log('checkForSubscription()');
    const context = this;
    return new TransformStream({
      /**
       * @param {Event} chunk
       */
      transform(chunk, controller) {
        if (context.observationStartIndex) {
          if (
            chunk.streamEventIndex &&
            chunk.streamEventIndex > context.observationStartIndex
          ) {
            // chunk.Events
            // // Filter out events that are older than our observationStartTime
            // .filter(
            //   /**
            //    * @param {Event} event
            //    */
            //   (event) => {
            //   return event.streamEventIndex > context.observationStartIndex;
            // })
            // .forEach(
            //   /**
            //    * @param {Event} event
            //   **/
            //   (event) => {
            context.subscriptions
              .filter((subscription) => !subscription.muted)
              .forEach((subscription) => {
                // console.log('forEachsub', subscription.Topic, chunk.streamEventTopic)
                if (subscription.Topic === chunk.streamEventTopic) {
                  let matches = subscription.conditions.every((condition) => {
                    return context.eventMatchesCondition(
                      chunk,
                      condition,
                      subscription.Topic
                    );
                  });

                  if (matches) {
                    console.log('=+=+=+=+=+=Subscription match found:', chunk);
                    context.notifications.add({
                      title: `${chunk.streamEventTopic} Notification`,
                      message: `Subscription match found: ${
                        chunk.streamEventTopic
                      } ${chunk.DisplayMessage || chunk.Message}`,
                      color: subscription.notificationType || 'highlight',
                      sticky: true,
                      customAction: {
                        label: 'Log Event',
                        action: () => {
                          console.log('event', chunk);
                        },
                      },
                    });

                    if (subscription.playSound) {
                      context.beep();
                    }
                  } else {
                    // console.log('no match for', chunk);
                  }
                }
                // });
              });
          }
        }
        controller.enqueue(chunk); // Enqueue the chunk unchanged
      },
    });
  }

  /**
   * @param {Event} event
   * @param {EventSubscriptionCondition} condition
   * @param {"Allocation" | "Evaluation" | "Node"} topic
   * @returns {boolean}
   */
  eventMatchesCondition(event, condition, topic) {
    // console.log('event Matches Condition', event, condition, topic);
    switch (topic) {
      case 'Allocation':
        return this.allocationEventMatchesCondition(event, condition);
      case 'Node':
        return this.nodeEventMatchesCondition(event, condition);
      // case "Evaluation":
      //   return this.evaluationEventMatchesCondition(event, condition);
      default:
        return false;
    }
  }

  /**
   * @param {Event} event
   * @param {AllocationEventSubscriptionCondition} condition
   */
  allocationEventMatchesCondition(event, condition) {
    // Eliminate events that don't match the task/job
    // Return false if event job doesn't match condition
    // console.log('checking condition', event);
    if (
      !(condition.jobs.includes('*') || condition.jobs.includes(event.jobName))
    ) {
      return false;
    }

    if (
      !(
        condition.tasks.includes('*') ||
        condition.tasks.includes(event.taskName)
      )
    ) {
      return false;
    }

    if (condition.matchType === 'contains') {
      return event[condition.stringKey].includes(condition.value);
    } else if (condition.matchType === 'equals') {
      return event[condition.stringKey] === condition.value;
    }

    // return true;
  }

  /**
   * @param {Event} event
   * @param {EventSubscriptionCondition} condition
   * @returns {boolean}
   **/
  nodeEventMatchesCondition(event, condition) {
    if (condition.matchType === 'contains') {
      return event[condition.stringKey].includes(condition.value);
    } else if (condition.matchType === 'equals') {
      return event[condition.stringKey] === condition.value;
    }
  }

  beep() {
    // var snd = new Audio("data:audio/wav;base64,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qJrx4KOQjahZxWbcZgztj2c49nKmkId44S71j0c8eV9yDK6uPRzx5X18eDvjvQ6yKo9ZSS6l//8elePK/Lf//IInrOF/FvDoADYAGBMGb7FtErm5MXMlmPAJQVgWta7Zx2go+8xJ0UiCb8LHHdftWyLJE0QIAIsI+UbXu67dZMjmgDGCGl1H+vpF4NSDckSIkk7Vd+sxEhBQMRU8j/12UIRhzSaUdQ+rQU5kGeFxm+hb1oh6pWWmv3uvmReDl0UnvtapVaIzo1jZbf/pD6ElLqSX+rUmOQNpJFa/r+sa4e/pBlAABoAAAAA3CUgShLdGIxsY7AUABPRrgCABdDuQ5GC7DqPQCgbbJUAoRSUj+NIEig0YfyWUho1VBBBA//uQZB4ABZx5zfMakeAAAAmwAAAAF5F3P0w9GtAAACfAAAAAwLhMDmAYWMgVEG1U0FIGCBgXBXAtfMH10000EEEEEECUBYln03TTTdNBDZopopYvrTTdNa325mImNg3TTPV9q3pmY0xoO6bv3r00y+IDGid/9aaaZTGMuj9mpu9Mpio1dXrr5HERTZSmqU36A3CumzN/9Robv/Xx4v9ijkSRSNLQhAWumap82WRSBUqXStV/YcS+XVLnSS+WLDroqArFkMEsAS+eWmrUzrO0oEmE40RlMZ5+ODIkAyKAGUwZ3mVKmcamcJnMW26MRPgUw6j+LkhyHGVGYjSUUKNpuJUQoOIAyDvEyG8S5yfK6dhZc0Tx1KI/gviKL6qvvFs1+bWtaz58uUNnryq6kt5RzOCkPWlVqVX2a/EEBUdU1KrXLf40GoiiFXK///qpoiDXrOgqDR38JB0bw7SoL+ZB9o1RCkQjQ2CBYZKd/+VJxZRRZlqSkKiws0WFxUyCwsKiMy7hUVFhIaCrNQsKkTIsLivwKKigsj8XYlwt/WKi2N4d//uQRCSAAjURNIHpMZBGYiaQPSYyAAABLAAAAAAAACWAAAAApUF/Mg+0aohSIRobBAsMlO//Kk4soosy1JSFRYWaLC4qZBYWFRGZdwqKiwkNBVmoWFSJkWFxX4FFRQWR+LsS4W/rFRb/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////VEFHAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAU291bmRib3kuZGUAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAMjAwNGh0dHA6Ly93d3cuc291bmRib3kuZGUAAAAAAAAAACU=");
    var snd = new Audio(
      'data:audio/mpeg;base64,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hIXMkGWhZ4x1kygIjIXLOAwbAiWYEl+gQsWAaSJLo0taf6njUulMOxBxniZOZigjJmAeSAxaLKVSA4KDQSJIpVM3TFYwx9AKjspeoM/aJrNASJJM0PBgDqoVUJWLUroSSL+kIkIzIlY5mSioZzIZmgadhfJyy6ypzUMZEl2xRMJI1ThDFWgwkSPEgsOj8kh6MF3hwpWIsk46VKXIJUnQZQLTQVbggCQ6kQTChSgDGWDAxRkxQoxDVUS6ZVDcgcqPu7OOFCkblfmIaHgcZI1NVVZi6YyiqwiNRclWZi7LakZuSqnf2tUYMXJAiyFwBCURLpJyswS6AhSAqFjvTNnVrZoobFhZJWCppo2aFliw2aaNi5VMQU1FMy4xMDBVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVVV'
    );
    snd.play();
  }

  /**
   * @param {EventSubscription} subscription
   */
  @action removeSubscription(subscription) {
    this.subscriptions.removeObject(subscription);
  }

  /**
   * @param {EventSubscription} subscription
   */
  @action editSubscription(subscription) {
    this.subscriptionBeingEdited = subscription;
  }

  @tracked subscriptionBeingEdited = null;

  notificationTypes = [
    'critical',
    'warning',
    'success',
    'highlight',
    'neutral',
  ];

  /**
   * @param {EventSubscription} subscription
   * @param {string} propertyName
   * @param {InputEvent} event // TODO: close enough for a hackathon
   */
  @action setSubscriptionProperty(
    subscription,
    propertyName,
    value = null,
    event
  ) {
    subscription[propertyName] = value !== null ? value : event.target.value;
  }

  //#endregion Subscriptions
}
