import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as controller } from '@ember/controller';
import { task } from 'ember-concurrency';

export default class OptimizeController extends Controller {
  @controller('optimize/summary') summaryController;

  get activeRecommendationSummary() {
    return this.summaryController.model;
  }

  // This is a task because the accordion uses timeouts for animation
  // eslint-disable-next-line require-yield
  @(task(function*() {
    const currentSummaryIndex = this.model.indexOf(this.activeRecommendationSummary);
    const nextSummary = this.model.objectAt(currentSummaryIndex + 1);

    if (nextSummary) {
      this.transitionToSummary(nextSummary);
    } else {
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
