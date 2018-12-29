import { alias } from '@ember/object/computed';
import Controller, { inject as controller } from '@ember/controller';

export default Controller.extend({
  serversController: controller('servers'),
  isForbidden: alias('serversController.isForbidden'),
});
