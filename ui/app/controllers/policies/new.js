// @ts-check

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';

export default class PoliciesNewController extends Controller {
  @service store;
  @service flashMessages;
  @service router;
}
