import Controller from '@ember/controller';

export default class ServersServerController extends Controller {
  get server() {
    return this.model;
  }
}
