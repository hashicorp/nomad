import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';
import { sort } from '@ember/object/computed';
import { task, timeout } from 'ember-concurrency';
import Ember from 'ember';

export default class OptimizeController extends Controller {
  @tracked recommendationSummaryIndex = 0;

  summarySorting = ['submitTime:desc'];
  @sort('model', 'summarySorting') sortedSummaries;

  get activeRecommendationSummary() {
    return this.sortedSummaries.objectAt(this.recommendationSummaryIndex);
  }

  @(task(function*() {
    this.recommendationSummaryIndex++;

    if (this.recommendationSummaryIndex >= this.model.length) {
      this.store.unloadAll('recommendation-summary');
      yield timeout(Ember.testing ? 0 : 1000);
      this.store.findAll('recommendation-summary');
    }
  }).drop())
  proceed;
}
