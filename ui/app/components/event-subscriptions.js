// @ts-check

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class EventSubscriptionsComponent extends Component {
  @service events;

  constructor() {
    super(...arguments);
    this.events.start();
  }

  deactivateFlyout() {
    this.events.sidebarIsActive = false;
  }

  logEvent(event) {
    console.log('event', event);
  }
}
