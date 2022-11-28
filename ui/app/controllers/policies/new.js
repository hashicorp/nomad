// @ts-check

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class PoliciesNewController extends Controller {
  @service store;
  @service flashMessages;
  @service router;

}
