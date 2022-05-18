import Controller from '@ember/controller';
import { action } from '@ember/object';

export default class VariablesNewController extends Controller {
  @action
  async saveNewVariable({ path, key, value }) {
    console.log('abacus', path, key, value);
    window.store = this.store;
    // let newVar = this.store.createRecord('var', { id: path, key, value });
    // console.log({newVar});
    // newVar.save();
    let newVar = this.store.push({
      data: [
        {
          id: path,
          type: 'var',
          attributes: {
            key,
            value,
          },
        },
      ],
    });

    this.store.peekRecord('var', path).save();

    // console.log(this.store.findAll('var')[0]);
    //.save();
  }
}
