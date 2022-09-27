// @ts-check

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class BreadcrumbsPeersComponent extends Component {
  @service router;
  selectedPeer = {};
  selectPeer(peer) {
    const [routeName, ...models] = peer.args;
    this.router.transitionTo(routeName, ...models);
  }
}
