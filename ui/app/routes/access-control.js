import Route from '@ember/routing/route';

export default class AccessControlRoute extends Route {
  // Load our tokens, roles, and policies
  async model() {
    return {
      tokens: await this.store.findAll('token'),
      roles: await this.store.findAll('role'),
      policies: await this.store.findAll('policy'),
    };
  }
}
