// @ts-check

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class EventSubscriptionsComponent extends Component {
  @service events;

  constructor() {
    super(...arguments);
    console.log('event subscriptions', this.events);
    this.events.start();
  }

  deactivateFlyout() {
    console.log('deactivate flyout', this.events);
    this.events.sidebarIsActive = false;
  }

  logEvent(event) {
    console.log('event', event);
  }
}
