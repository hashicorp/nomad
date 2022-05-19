import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
export default class VariablesNewController extends Controller {
  @service router;

  @action
  saveNewVariable({ path, key, value }) {
    let props = { id: path, path, items: { [key]: value } };
    this.model.setProperties(props);
    this.model.save().then(() => {
      this.router.transitionTo('variables');
    });
  }
}
