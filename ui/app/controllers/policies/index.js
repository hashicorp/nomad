import Controller, { inject as controller } from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class PoliciesIndexController extends Controller {
  get policies() {
    return this.model;
  }
}
