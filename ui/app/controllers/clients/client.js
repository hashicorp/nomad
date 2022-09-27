import Controller from '@ember/controller';
import { inject as service } from '@ember/service';

export default class ClientsClientController extends Controller {
  @service store;

  get client() {
    return this.model;
  }

  get peers() {
    return this.store
      .peekAll('node')
      .rejectBy('id', this.client.id)
      .map((node) => {
        return {
          label: node.get('shortId'),
          args: ['clients.client', node.get('id')],
        };
      });
  }

  get breadcrumb() {
    return {
      title: 'Client',
      label: this.client.get('shortId'),
      args: ['clients.client', this.client.get('id')],
      peers: this.peers,
    };
  }
}
