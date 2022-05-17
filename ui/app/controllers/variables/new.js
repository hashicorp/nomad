import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class VariablesNewController extends Controller {
  @tracked key;
  @tracked value;
  @tracked path;

  @action
  saveNewVariable(e) {
    e.preventDefault();
    console.log('Creating new variable:', this.path, this.key, this.value);
  }
}
