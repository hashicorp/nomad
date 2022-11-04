// @ts-check
import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class SettingsTokensRoute extends Route {
  @service store;
  model() {
    console.log('Tokin', this.store.peekAll('auth-method'));
    return {
      authMethods: this.store.findAll('auth-method'),
    };
  }
}
