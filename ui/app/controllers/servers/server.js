import Controller from '@ember/controller';

export default class ServersServerController extends Controller {
  get server() {
    return this.model;
  }

  get breadcrumb() {
    return {
      title: 'Server',
      label: this.server.name,
      args: ['servers.server', this.server.id],
    };
  }
}
