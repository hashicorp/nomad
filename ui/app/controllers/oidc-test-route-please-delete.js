import Controller from '@ember/controller';
import { action } from '@ember/object';
import addToPath from 'nomad-ui/utils/add-to-path';

export default class OidcTestRoutePleaseDeleteController extends Controller {
  queryParams = ['auth_method', 'client_nonce', 'redirect_uri', 'meta'];

  @action
  doGood() {
    window.location = addToPath(this.redirect_uri, `?code=${this.fakeUserName}&state=success`);
  }

  @action
  doBad() {
    window.location = addToPath(this.redirect_uri, '?state=failure');
  }


}
