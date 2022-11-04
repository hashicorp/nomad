import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class OidcTestRoutePleaseDeleteController extends Controller {
  queryParams = ['auth_method', 'client_nonce', 'redirect_uri', 'meta']

  @action
  doGood() {
    window.location = `${this.redirect_uri}?fakeRedirect=success`;
  }

  @action
  doBad() {
    window.location = `${this.redirect_uri}?fakeRedirect=failure`;
  }
}
