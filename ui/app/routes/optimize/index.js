import Route from '@ember/routing/route';

export default class OptimizeIndexRoute extends Route {
  async redirect() {
    const summaries = this.modelFor('optimize');

    if (summaries.length) {
      const firstSummary = summaries.objectAt(0);

      return this.transitionTo('optimize.summary', firstSummary.slug, {
        queryParams: { jobNamespace: firstSummary.jobNamespace || 'default' },
      });
    }
  }
}
