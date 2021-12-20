import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class EvaluationsController extends Controller {
  queryParams = ['pageSize'];

  get evaluations() {
    return this.model;
  }

  @tracked pageSize = 25;

  @action
  onChange(newPageSize) {
    this.pageSize = newPageSize;
  }
}
