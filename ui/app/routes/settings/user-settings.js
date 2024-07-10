import Route from '@ember/routing/route';

export default class SettingsUserSettingsRoute extends Route {
  // Make sure to load namespaces
  model() {
    return {
      namespaces: this.store.findAll('namespace'),
      nodePools: this.store.findAll('node-pool'),
    };
  }
}
