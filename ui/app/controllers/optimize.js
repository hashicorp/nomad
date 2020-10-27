import Controller from '@ember/controller';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { task, timeout } from 'ember-concurrency';
import Ember from 'ember';

export default class OptimizeController extends Controller {
  @service router;

  get activeRecommendationSummary() {
    const currentRoute = this.router.currentRoute;

    if (currentRoute.name === 'optimize.summary') {
      return this.model.findBy('slug', currentRoute.params.slug);
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
      this.store.unloadAll('recommendation-summary');
      yield timeout(Ember.testing ? 0 : 1000);
      yield this.store.findAll('recommendation-summary');
      this.send('reachedEnd');
    }
  }).drop())
  proceed;

  @action
  transitionToSummary(summary) {
    this.transitionToRoute('optimize.summary', summary.slug);
  }
}
