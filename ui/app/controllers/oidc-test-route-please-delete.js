import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class OidcTestRoutePleaseDeleteController extends Controller {
  queryParams = ['auth_method', 'client_nonce', 'redirect_uri', 'meta'];

  @action
  signIn(fakeAccount) {
    window.location = `${this.redirect_uri.split('?')[0]}?code=${
      fakeAccount.secret
    }&state=success`;
  }

  @action
  failToSignIn() {
    window.location = `${this.redirect_uri.split('?')[0]}?state=failure`;
  }
}
