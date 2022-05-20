import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
export default class VariablesNewController extends Controller {
  @service router;

  @action
  saveNewVariable({ path, keyValues }) {
    // TODO: validation

    // Transform key value array into object
    const items = keyValues.reduce((acc, { key, value }) => {
      acc[key] = value;
      return acc;
    }, {});

    let props = { id: path, path, items };
    this.model.setProperties(props);
    this.model.save().then(() => {
      this.router.transitionTo('variables');
    });
  }
}
