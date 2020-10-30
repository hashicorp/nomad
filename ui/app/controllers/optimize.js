import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as controller } from '@ember/controller';
import { task, timeout } from 'ember-concurrency';

export default class OptimizeController extends Controller {
  @controller('optimize/summary') summaryController;

  get activeRecommendationSummary() {
    return this.summaryController.model;
  }

  @(task(function*() {
    const currentSummaryIndex = this.model.indexOf(this.activeRecommendationSummary);
    const nextSummary = this.model.objectAt(currentSummaryIndex + 1);

    if (nextSummary) {
      this.transitionToSummary(nextSummary);
    } else {
      // This is a task because the accordion has actual timeouts for animation
      yield timeout(0);
      this.send('reachedEnd');
    }
  }).drop())
  proceed;

  @action
  transitionToSummary(summary) {
    this.transitionToRoute('optimize.summary', summary.slug, {
      queryParams: { jobNamespace: summary.jobNamespace },
    });
  }
}
