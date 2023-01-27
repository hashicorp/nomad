import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class JobsRunTemplatesIndexController extends Controller {
  @tracked selectedTemplate = null;

  @action
  onChange(e) {
    this.selectedTemplate = e.target.id;
  }
}
