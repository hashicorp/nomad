import Controller from '@ember/controller';
import { task } from 'ember-concurrency';
import { inject as service } from '@ember/service';

export default class AccessControlTokensIndexController extends Controller {
  @service notifications;

  @task(function* (token) {
    try {
      yield token.deleteRecord();
      yield token.save();
      this.notifications.add({
        title: `Token ${token.name} successfully deleted`,
        color: 'success',
      });
    } catch (err) {
      this.error = {
        title: 'Error deleting token',
        description: err,
      };

      throw err;
    }
  })
  deleteToken;
}
