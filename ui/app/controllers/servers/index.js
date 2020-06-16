import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';

export default class IndexController extends Controller {
  @controller('servers') serversController;
  @alias('serversController.isForbidden') isForbidden;
}
