import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class EvaluationsController extends Controller {
  queryParams = ['nextToken', 'pageSize'];

  get shouldDisableNext() {
    return !this.model.meta?.nextToken;
  }

  get shouldDisablePrev() {
    return !this.previousTokens.length;
  }

  @tracked pageSize = 25;
  @tracked nextToken = null;
  @tracked previousTokens = [];

  @action
  onChange(newPageSize) {
    this.pageSize = newPageSize;
  }

  @action
  onNext(nextToken) {
    this.previousTokens = [...this.previousTokens, this.nextToken];
    this.nextToken = nextToken;
  }

  @action
  onPrev(lastToken) {
    this.previousTokens.pop();
    this.previousTokens = [...this.previousTokens];
    this.nextToken = lastToken;
  }

  @action
  refresh() {
    this.nextToken = null;
    this.previousTokens = [];
  }
}
