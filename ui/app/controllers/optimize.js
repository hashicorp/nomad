import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { task, timeout } from 'ember-concurrency';

export default class OptimizeController extends Controller {
  @service router;

  get activeRecommendationSummary() {
    const currentRoute = this.router.currentRoute;

    if (currentRoute.name === 'optimize.summary') {
      return this.model.find(
        summary =>
          summary.slug === currentRoute.params.slug &&
          summary.jobNamespace === currentRoute.queryParams.namespace
      );
    } else {
      return undefined;
    }
  }

  @(task(function*() {
    const currentSummaryIndex = this.model.indexOf(this.activeRecommendationSummary);
    const nextSummary = this.model.objectAt(currentSummaryIndex + 1);

    if (nextSummary) {
      this.transitionToSummary(nextSummary);
    } else {
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
